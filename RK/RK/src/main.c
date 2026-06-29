#include <fcntl.h>
#include <linux/fb.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

static uint32_t le32(const unsigned char *p) {
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8) | ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}

static uint16_t le16(const unsigned char *p) {
    return (uint16_t)p[0] | ((uint16_t)p[1] << 8);
}

static ssize_t read_full(int fd, unsigned char *buf, size_t len) {
    size_t done = 0;
    while (done < len) {
        ssize_t n = read(fd, buf + done, len - done);
        if (n < 0) {
            if (errno == EINTR) {
                continue;
            }
            return -1;
        }
        if (n == 0) {
            break;
        }
        done += (size_t)n;
    }
    return (ssize_t)done;
}

static ssize_t write_full(int fd, const unsigned char *buf, size_t len) {
    size_t done = 0;
    while (done < len) {
        ssize_t n = write(fd, buf + done, len - done);
        if (n < 0) {
            if (errno == EINTR) {
                continue;
            }
            return -1;
        }
        if (n == 0) {
            break;
        }
        done += (size_t)n;
    }
    return (ssize_t)done;
}

static uint32_t pack_pixel(struct fb_var_screeninfo *vinfo, unsigned char r, unsigned char g, unsigned char b) {
    uint32_t pixel = 0;
    if (vinfo->red.length) {
        pixel |= ((uint32_t)(r >> (8 - vinfo->red.length)) << vinfo->red.offset);
    }
    if (vinfo->green.length) {
        pixel |= ((uint32_t)(g >> (8 - vinfo->green.length)) << vinfo->green.offset);
    }
    if (vinfo->blue.length) {
        pixel |= ((uint32_t)(b >> (8 - vinfo->blue.length)) << vinfo->blue.offset);
    }
    if (vinfo->transp.length) {
        pixel |= ((uint32_t)0xff >> (8 - vinfo->transp.length)) << vinfo->transp.offset;
    }
    return pixel;
}

int main(int argc, char **argv) {
    if (argc > 1 && (strcmp(argv[1], "-h") == 0 || strcmp(argv[1], "--help") == 0)) {
        fprintf(stderr, "usage: %s [image_24bit.bmp]\n", argv[0]);
        return 0;
    }

    const char *bmp_path = (argc > 1) ? argv[1] : "/userdata/vision/res/flag.bmp";
    setvbuf(stderr, NULL, _IONBF, 0);
    fprintf(stderr, "[main] bmp=%s\n", bmp_path);

    int lcd_fd = open("/dev/fb0", O_RDWR);
    if (lcd_fd < 0) {
        perror("open /dev/fb0");
        return 1;
    }
    fprintf(stderr, "[main] /dev/fb0 opened fd=%d\n", lcd_fd);

    struct fb_var_screeninfo vinfo;
    struct fb_fix_screeninfo finfo;
    if (ioctl(lcd_fd, FBIOGET_VSCREENINFO, &vinfo) < 0 || ioctl(lcd_fd, FBIOGET_FSCREENINFO, &finfo) < 0) {
        perror("ioctl fb");
        close(lcd_fd);
        return 1;
    }
    fprintf(stderr,
            "[main] fb: %ux%u bpp=%u line_length=%u\n",
            vinfo.xres, vinfo.yres, vinfo.bits_per_pixel, finfo.line_length);
    fprintf(stderr,
            "[main] fb offset: x=%u y=%u virtual=%ux%u\n",
            vinfo.xoffset, vinfo.yoffset, vinfo.xres_virtual, vinfo.yres_virtual);

    int bmp_fd = open(bmp_path, O_RDONLY);
    if (bmp_fd < 0) {
        perror("open bmp");
        close(lcd_fd);
        return 1;
    }
    fprintf(stderr, "[main] bmp opened fd=%d\n", bmp_fd);

    unsigned char header[54];
    if (read(bmp_fd, header, sizeof(header)) != (ssize_t)sizeof(header) || header[0] != 'B' || header[1] != 'M') {
        fprintf(stderr, "invalid bmp file\n");
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }

    uint32_t data_offset = le32(header + 10);
    int bmp_w = (int)le32(header + 18);
    int bmp_h = (int)le32(header + 22);
    uint16_t bmp_bpp = le16(header + 28);
    fprintf(stderr, "[main] bmp: w=%d h=%d bpp=%u data_offset=%u\n", bmp_w, bmp_h, bmp_bpp, data_offset);
    if (bmp_w <= 0 || bmp_h == 0 || bmp_bpp != 24) {
        fprintf(stderr, "only 24-bit bmp supported\n");
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }

    int top_down = bmp_h < 0;
    if (bmp_h < 0) {
        bmp_h = -bmp_h;
    }

    if (data_offset > sizeof(header)) {
        off_t skip = (off_t)data_offset - (off_t)sizeof(header);
        if (lseek(bmp_fd, skip, SEEK_CUR) < 0) {
            perror("lseek");
            close(bmp_fd);
            close(lcd_fd);
            return 1;
        }
    }

    int row_bytes = ((bmp_w * 3 + 3) / 4) * 4;
    fprintf(stderr, "[main] bmp row_bytes=%d\n", row_bytes);
    unsigned char *row = malloc((size_t)row_bytes);
    if (!row) {
        perror("malloc");
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }

    int lcd_w = (int)vinfo.xres;
    int lcd_h = (int)vinfo.yres;
    int bytes_per_pixel = (int)vinfo.bits_per_pixel / 8;
    if (bytes_per_pixel != 4) {
        fprintf(stderr, "lcd bpp=%u not supported by this demo\n", vinfo.bits_per_pixel);
        free(row);
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }

    size_t fb_size = (size_t)finfo.line_length * vinfo.yres;
    unsigned char *frame = malloc(fb_size);
    if (!frame) {
        perror("malloc frame");
        free(row);
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }

    off_t visible_offset = (off_t)vinfo.yoffset * (off_t)finfo.line_length;
    if (lseek(lcd_fd, visible_offset, SEEK_SET) < 0) {
        perror("lseek fb visible");
        memset(frame, 0, fb_size);
    } else {
        ssize_t n = read_full(lcd_fd, frame, fb_size);
        if (n != (ssize_t)fb_size) {
            if (n < 0) {
                perror("read fb visible");
                n = 0;
            } else {
                fprintf(stderr, "[main] short framebuffer read: %zd/%zu\n", n, fb_size);
            }
            memset(frame + (n > 0 ? (size_t)n : 0), 0, fb_size - (n > 0 ? (size_t)n : 0));
        }
    }

    int x0 = (lcd_w - bmp_w) / 2;
    int y0 = (lcd_h - bmp_h) / 2;
    if (x0 < 0) x0 = 0;
    if (y0 < 0) y0 = 0;
    fprintf(stderr, "[main] dst offset x0=%d y0=%d\n", x0, y0);

    int bmp_ok = 1;
    for (int y = 0; y < bmp_h; ++y) {
        if (read_full(bmp_fd, row, (size_t)row_bytes) != row_bytes) {
            fprintf(stderr, "bmp read error\n");
            bmp_ok = 0;
            break;
        }
        int dy = y0 + (top_down ? y : (bmp_h - 1 - y));
        if (dy >= lcd_h) {
            continue;
        }
        for (int x = 0; x < bmp_w; ++x) {
            int dx = x0 + x;
            if (dx >= lcd_w) {
                continue;
            }
            unsigned char *src = row + x * 3;
            uint32_t *dst = (uint32_t *)(frame + (size_t)dy * finfo.line_length);
            dst[dx] = pack_pixel(&vinfo, src[2], src[1], src[0]);
        }
    }
    if (!bmp_ok) {
        free(frame);
        free(row);
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }

    if (lseek(lcd_fd, 0, SEEK_SET) < 0) {
        perror("lseek fb");
        free(frame);
        free(row);
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }
    if (write_full(lcd_fd, frame, fb_size) != (ssize_t)fb_size) {
        perror("write fb");
        free(frame);
        free(row);
        close(bmp_fd);
        close(lcd_fd);
        return 1;
    }
    vinfo.xoffset = 0;
    vinfo.yoffset = 0;
    vinfo.activate = FB_ACTIVATE_NOW;
    if (ioctl(lcd_fd, FBIOPAN_DISPLAY, &vinfo) < 0) {
        perror("FBIOPAN_DISPLAY");
    }
    if (ioctl(lcd_fd, FBIOBLANK, FB_BLANK_UNBLANK) < 0) {
        perror("FBIOBLANK");
    }

    free(frame);
    free(row);
    close(bmp_fd);
    close(lcd_fd);
    fprintf(stderr, "[main] done\n");
    return 0;
}
