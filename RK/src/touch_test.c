#include <errno.h>
#include <fcntl.h>
#include <linux/input.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <time.h>
#include <unistd.h>

static volatile sig_atomic_t running = 1;

static void on_sigint(int sig) {
    (void)sig;
    running = 0;
}

static const char *region_name(int x, int y, int width, int height) {
    int third = width / 3;
    if (x < third) {
        return "left_button";
    }
    if (x < third * 2) {
        return "middle_button";
    }
    (void)y;
    (void)height;
    return "right_button";
}

static const char *swipe_name(int dx, int dy) {
    int adx = dx < 0 ? -dx : dx;
    int ady = dy < 0 ? -dy : dy;
    if (adx < 80 && ady < 80) {
        return "tap";
    }
    if (adx >= ady) {
        return dx > 0 ? "swipe_right" : "swipe_left";
    }
    return dy > 0 ? "swipe_down" : "swipe_up";
}

static void usage(const char *argv0) {
    fprintf(stderr, "usage: %s [--seconds n] [/dev/input/event6] [screen_width screen_height]\n", argv0);
}

int main(int argc, char **argv) {
    const char *dev = "/dev/input/event6";
    int width = 1024;
    int height = 600;
    int seconds = 0;
    int pos = 0;
    for (int i = 1; i < argc; ++i) {
        if (strcmp(argv[i], "-h") == 0 || strcmp(argv[i], "--help") == 0) {
            usage(argv[0]);
            return 0;
        }
        if (strcmp(argv[i], "--seconds") == 0 && i + 1 < argc) {
            seconds = atoi(argv[++i]);
            continue;
        }
        if (pos == 0) {
            dev = argv[i];
        } else if (pos == 1) {
            width = atoi(argv[i]);
        } else if (pos == 2) {
            height = atoi(argv[i]);
        } else {
            usage(argv[0]);
            return 2;
        }
        ++pos;
    }

    int fd = open(dev, O_RDONLY);
    if (fd < 0) {
        perror(dev);
        return 1;
    }

    signal(SIGINT, on_sigint);
    printf("touch device: %s, screen=%dx%d\n", dev, width, height);
    if (seconds > 0) {
        printf("auto exit after %d seconds\n", seconds);
    } else {
        printf("press Ctrl+C to exit\n");
    }

    int x = 0;
    int y = 0;
    int down = 0;
    int start_x = 0;
    int start_y = 0;
    struct input_event ev;

    time_t deadline = seconds > 0 ? time(NULL) + seconds : 0;
    while (running) {
        if (deadline && time(NULL) >= deadline) {
            break;
        }
        fd_set fds;
        struct timeval tv;
        FD_ZERO(&fds);
        FD_SET(fd, &fds);
        tv.tv_sec = 1;
        tv.tv_usec = 0;
        int ready = select(fd + 1, &fds, NULL, NULL, &tv);
        if (ready < 0) {
            if (errno == EINTR) {
                continue;
            }
            perror("select input_event");
            break;
        }
        if (ready == 0) {
            continue;
        }

        ssize_t n = read(fd, &ev, sizeof(ev));
        if (n < 0) {
            if (errno == EINTR) {
                continue;
            }
            perror("read input_event");
            break;
        }
        if (n != sizeof(ev)) {
            continue;
        }

        if (ev.type == EV_ABS) {
            if (ev.code == ABS_X || ev.code == ABS_MT_POSITION_X) {
                x = ev.value;
            } else if (ev.code == ABS_Y || ev.code == ABS_MT_POSITION_Y) {
                y = ev.value;
            }
        } else if (ev.type == EV_KEY && ev.code == BTN_TOUCH) {
            if (ev.value) {
                down = 1;
                start_x = x;
                start_y = y;
                printf("down x=%d y=%d region=%s\n", x, y, region_name(x, y, width, height));
            } else if (down) {
                int dx = x - start_x;
                int dy = y - start_y;
                printf("up x=%d y=%d region=%s gesture=%s dx=%d dy=%d\n",
                       x, y, region_name(x, y, width, height), swipe_name(dx, dy), dx, dy);
                down = 0;
            }
            fflush(stdout);
        } else if (ev.type == EV_SYN && down) {
            printf("move x=%d y=%d region=%s\n", x, y, region_name(x, y, width, height));
            fflush(stdout);
        }
    }

    close(fd);
    return 0;
}
