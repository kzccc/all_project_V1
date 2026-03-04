#include <linux/module.h>
#include <linux/fs.h>
#include <linux/init.h>
#include <linux/pagemap.h>
#include <linux/kfifo.h>
#include <linux/slab.h>
#include <linux/uaccess.h>

#define MYFS_MAGIC 0x20241229
#define MYFS_FIFO_SIZE 4096

struct myfs_sb_info {
    struct kfifo fifo;
    spinlock_t lock;
};

static struct file_system_type myfs_type;

static int myfs_file_open(struct inode *inode, struct file *file)
{
    return 0;
}

static ssize_t myfs_file_write(struct file *file,
                               const char __user *buf,
                               size_t len, loff_t *ppos)
{
    struct myfs_sb_info *sbi = file->f_inode->i_sb->s_fs_info;
    unsigned int copied;
    int ret;

    spin_lock(&sbi->lock);
    ret = kfifo_from_user(&sbi->fifo, buf, len, &copied);
    spin_unlock(&sbi->lock);

    return ret ? ret : copied;
}

static ssize_t myfs_file_read(struct file *file,
                              char __user *buf,
                              size_t len, loff_t *ppos)
{
    struct myfs_sb_info *sbi = file->f_inode->i_sb->s_fs_info;
    unsigned int copied;
    int ret;

    spin_lock(&sbi->lock);
    ret = kfifo_to_user(&sbi->fifo, buf, len, &copied);
    spin_unlock(&sbi->lock);

    return ret ? ret : copied;
}

static const struct file_operations myfs_file_ops = {
    .owner = THIS_MODULE,
    .open  = myfs_file_open,
    .read  = myfs_file_read,
    .write = myfs_file_write,
};

static struct inode *myfs_get_inode(struct super_block *sb,
                                    const struct inode *dir,
                                    umode_t mode)
{
    struct inode *inode = new_inode(sb);
    if (!inode)
        return NULL;

    inode->i_ino = get_next_ino();
    inode_init_owner(inode, dir, mode);
    inode->i_atime = inode->i_mtime = inode->i_ctime = current_time(inode);

    if (S_ISDIR(mode)) {
        inode->i_op = &simple_dir_inode_operations;
        inode->i_fop = &simple_dir_operations;
        inc_nlink(inode);
    } else if (S_ISREG(mode)) {
        inode->i_fop = &myfs_file_ops;
    }
    return inode;
}

static int myfs_mknod(struct inode *dir, struct dentry *dentry, umode_t mode)
{
    struct inode *inode = myfs_get_inode(dir->i_sb, dir, mode);
    if (!inode)
        return -ENOMEM;

    d_add(dentry, inode);
    inc_nlink(dir);
    return 0;
}

static int myfs_mkdir(struct inode *dir, struct dentry *dentry, umode_t mode)
{
    return myfs_mknod(dir, dentry, mode | S_IFDIR);
}

static int myfs_create(struct inode *dir, struct dentry *dentry,
                       umode_t mode, bool excl)
{
    return myfs_mknod(dir, dentry, mode | S_IFREG);
}

static const struct inode_operations myfs_dir_inode_ops = {
    .lookup = simple_lookup,
    .mkdir  = myfs_mkdir,
    .create = myfs_create,
};

static int myfs_fill_super(struct super_block *sb, void *data, int silent)
{
    struct inode *root;
    struct myfs_sb_info *sbi;
    int ret;

    sb->s_magic = MYFS_MAGIC;

    sbi = kzalloc(sizeof(*sbi), GFP_KERNEL);
    if (!sbi)
        return -ENOMEM;

    spin_lock_init(&sbi->lock);
    ret = kfifo_alloc(&sbi->fifo, MYFS_FIFO_SIZE, GFP_KERNEL);
    if (ret) {
        kfree(sbi);
        return ret;
    }
    sb->s_fs_info = sbi;

    root = myfs_get_inode(sb, NULL, S_IFDIR | 0755);
    root->i_op = &myfs_dir_inode_ops;

    sb->s_root = d_make_root(root);
    return 0;
}

static struct dentry *myfs_mount(struct file_system_type *fs_type,
                                 int flags, const char *dev_name,
                                 void *data)
{
    return mount_nodev(fs_type, flags, data, myfs_fill_super);
}

static void myfs_kill_sb(struct super_block *sb)
{
    struct myfs_sb_info *sbi = sb->s_fs_info;
    kfifo_free(&sbi->fifo);
    kfree(sbi);
    kill_litter_super(sb);
}

static struct file_system_type myfs_type = {
    .owner   = THIS_MODULE,
    .name    = "myfs",
    .mount   = myfs_mount,
    .kill_sb = myfs_kill_sb,
};

static int __init myfs_init(void)
{
    return register_filesystem(&myfs_type);
}

static void __exit myfs_exit(void)
{
    unregister_filesystem(&myfs_type);
}

module_init(myfs_init);
module_exit(myfs_exit);
MODULE_LICENSE("GPL");
