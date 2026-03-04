#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>
#include <stdlib.h>

#define FILE_NAME "/mnt/myfs/one"

int main()
{
    int fd;
    char write_buf[] = "Hello myfs fifo\n";
    char read_buf[128];
    int ret;

    fd = open(FILE_NAME, O_CREAT | O_RDWR, 0666);
    if (fd < 0) {
        perror("open");
        return -1;
    }

    printf("open %s success\n", FILE_NAME);

    /* 写入数据 */
    ret = write(fd, write_buf, strlen(write_buf));
    if (ret < 0) {
        perror("write");
        close(fd);
        return -1;
    }

    printf("write %d bytes: %s\n", ret, write_buf);

    memset(read_buf, 0, sizeof(read_buf));

    /* 读取 FIFO 中的数据 */
    ret = read(fd, read_buf, sizeof(read_buf) - 1);
    if (ret < 0) {
        perror("read");
        close(fd);
        return -1;
    }

    printf("read %d bytes: %s\n", ret, read_buf);

    close(fd);
    return 0;
}
