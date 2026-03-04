#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>

int main()
{
    int fd = open("/mnt/myfs/hello", O_CREAT | O_RDWR, 0666);

    write(fd, "hello myfs", 10);

    char buf[32] = {0};
    lseek(fd, 0, SEEK_SET);
    read(fd, buf, 10);

    printf("read: %s\n", buf);
    close(fd);
    return 0;
}
