cmd_/workspace/chenzekai/mfs2/modules.order := {   echo /workspace/chenzekai/mfs2/mfs2.ko; :; } | awk '!x[$$0]++' - > /workspace/chenzekai/mfs2/modules.order
