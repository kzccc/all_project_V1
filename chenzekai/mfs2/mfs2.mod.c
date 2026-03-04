#include <linux/module.h>
#define INCLUDE_VERMAGIC
#include <linux/build-salt.h>
#include <linux/vermagic.h>
#include <linux/compiler.h>

BUILD_SALT;

MODULE_INFO(vermagic, VERMAGIC_STRING);
MODULE_INFO(name, KBUILD_MODNAME);

__visible struct module __this_module
__section(".gnu.linkonce.this_module") = {
	.name = KBUILD_MODNAME,
	.init = init_module,
#ifdef CONFIG_MODULE_UNLOAD
	.exit = cleanup_module,
#endif
	.arch = MODULE_ARCH_INIT,
};

#ifdef CONFIG_RETPOLINE
MODULE_INFO(retpoline, "Y");
#endif

static const struct modversion_info ____versions[]
__used __section("__versions") = {
	{ 0x9a37ddb6, "module_layout" },
	{ 0x6ef87610, "kill_litter_super" },
	{ 0x50d9aa88, "unregister_filesystem" },
	{ 0x8a5d1fc, "register_filesystem" },
	{ 0x13c49cc2, "_copy_from_user" },
	{ 0x636ed569, "inc_nlink" },
	{ 0xc6cb4199, "d_add" },
	{ 0xacda9812, "d_make_root" },
	{ 0xc11129cf, "set_nlink" },
	{ 0x993e9595, "simple_dir_operations" },
	{ 0x86468053, "simple_dir_inode_operations" },
	{ 0x71684bda, "kmem_cache_alloc_trace" },
	{ 0xe4a39d47, "kmalloc_caches" },
	{ 0xf3e3902d, "current_time" },
	{ 0xafbbd648, "inode_init_owner" },
	{ 0xe953b21f, "get_next_ino" },
	{ 0x18c6c536, "new_inode" },
	{ 0x6b10bee1, "_copy_to_user" },
	{ 0x88db9f48, "__check_object_size" },
	{ 0x718b284b, "mount_nodev" },
	{ 0x5b8239ca, "__x86_return_thunk" },
	{ 0xbdfb6dbb, "__fentry__" },
};

MODULE_INFO(depends, "");


MODULE_INFO(srcversion, "C0465ECAE5F43863C7A108E");
