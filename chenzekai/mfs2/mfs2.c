#include <linux/module.h>
#include <linux/fs.h>
#include <linux/init.h>
#include <linux/pagemap.h>
#include <linux/slab.h>
#include <linux/uaccess.h>

#define MFS2_MAGIC 0x20241230
#define MFS2_BUF_SIZE 4096

struct mfs2_inode_info {
    char buf[MFS2_BUF_SIZE];
    size_t size;
};

/* ================= file ops ================= */

static int mfs2_open(struct inode *inode, struct file *file)
{
    file->private_data = inode->i_private;
    return 0;
}

static ssize_t mfs2_read(struct file *file, char __user *buf,
                         size_t len, loff_t *ppos)
{
    struct mfs2_inode_info *info = file->private_data;

    if (*ppos >= info->size)
        return 0;

    if (*ppos + len > info->size)
        len = info->size - *ppos;

    if (copy_to_user(buf, info->buf + *ppos, len))
        return -EFAULT;

    *ppos += len;
    return len;
}

static ssize_t mfs2_write(struct file *file, const char __user *buf,
                          size_t len, loff_t *ppos)
{
    struct mfs2_inode_info *info = file->private_data;

    if (*ppos + len > MFS2_BUF_SIZE)
        len = MFS2_BUF_SIZE - *ppos;

    if (copy_from_user(info->buf + *ppos, buf, len))
        return -EFAULT;

    *ppos += len;
    if (info->size < *ppos)
        info->size = *ppos;

    return len;
}

static const struct file_operations mfs2_file_ops = {
    .open  = mfs2_open,
    .read  = mfs2_read,
    .write = mfs2_write,
};

/* ================= inode helpers ================= */

static struct inode *mfs2_make_inode(struct super_block *sb, umode_t mode)
{
    struct inode *inode = new_inode(sb);
    struct mfs2_inode_info *info;

    inode->i_ino = get_next_ino();
    inode_init_owner(inode, NULL, mode);
    inode->i_atime = inode->i_mtime = inode->i_ctime = current_time(inode);

    if (S_ISDIR(mode)) {
        inode->i_op  = &simple_dir_inode_operations;
        inode->i_fop = &simple_dir_operations;
        set_nlink(inode, 2);
    } else {
        inode->i_fop = &mfs2_file_ops;
        info = kzalloc(sizeof(*info), GFP_KERNEL);
        inode->i_private = info;
    }

    return inode;
}

/* ================= dir ops ================= */

static int mfs2_create(struct inode *dir, struct dentry *dentry,
                       umode_t mode, bool excl)
{
    struct inode *inode = mfs2_make_inode(dir->i_sb, S_IFREG | mode);
    d_add(dentry, inode);
    inc_nlink(dir);
    return 0;
}

static const struct inode_operations mfs2_dir_inode_ops = {
    .create = mfs2_create,
};

/* ================= super ================= */

static int mfs2_fill_super(struct super_block *sb, void *data, int silent)
{
    struct inode *root;

    sb->s_magic = MFS2_MAGIC;

    root = mfs2_make_inode(sb, S_IFDIR | 0755);
    root->i_op = &mfs2_dir_inode_ops;

    sb->s_root = d_make_root(root);
    return 0;
}

static struct dentry *mfs2_mount(struct file_system_type *fs_type,
                                 int flags, const char *dev,
                                 void *data)
{
    return mount_nodev(fs_type, flags, data, mfs2_fill_super);
}

static struct file_system_type mfs2_type = {
    .name    = "mfs2",
    .mount   = mfs2_mount,
    .kill_sb = kill_litter_super,
};

/* ================= init / exit ================= */

static int __init mfs2_init(void)
{
    return register_filesystem(&mfs2_type);
}

static void __exit mfs2_exit(void)
{
    unregister_filesystem(&mfs2_type);
}

module_init(mfs2_init);
module_exit(mfs2_exit);
MODULE_LICENSE("GPL");
