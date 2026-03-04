#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>

int main()
{
    int fd1 = open("/mnt/myfs/a", O_CREAT | O_RDWR, 0666);
    int fd2 = open("/mnt/myfs/b", O_CREAT | O_RDWR, 0666);

    if (fd1 < 0 || fd2 < 0) {
        perror("open");
        return 1;
    }

    write(fd1, "ABC", 3);
    write(fd2, "123", 3);
    write(fd1, "DEF", 3);

    char buf[16] = {0};
    int n;

    printf("read sequence: ");

    while ((n = read(fd2, buf, sizeof(buf)-1)) > 0) {
        buf[n] = 0;
        printf("%s", buf);
    }

    printf("\n");

    close(fd1);
    close(fd2);
    return 0;
}
