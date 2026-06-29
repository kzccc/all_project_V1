#include <errno.h>
#include <fcntl.h>
#include <linux/fb.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <unistd.h>

struct framebuffer {
    int fd;
    uint8_t *mem;
    size_t size;
    struct fb_var_screeninfo var;
    struct fb_fix_screeninfo fix;
};

static uint32_t rgb888(uint8_t r, uint8_t g, uint8_t b) {
    return ((uint32_t)r << 16) | ((uint32_t)g << 8) | b;
}

static uint32_t parse_color(const char *text) {
    if (!text || text[0] != '#') {
        return rgb888(0, 128, 255);
    }
    unsigned int value = 0;
    if (sscanf(text + 1, "%x", &value) != 1) {
        return rgb888(0, 128, 255);
    }
    return value & 0x00ffffffu;
}

static void put_pixel(struct framebuffer *fb, int x, int y, uint32_t rgb) {
    if (x < 0 || y < 0 || x >= (int)fb->var.xres || y >= (int)fb->var.yres) {
        return;
    }

    uint8_t r = (uint8_t)((rgb >> 16) & 0xff);
    uint8_t g = (uint8_t)((rgb >> 8) & 0xff);
    uint8_t b = (uint8_t)(rgb & 0xff);
    uint8_t *p = fb->mem + (size_t)y * fb->fix.line_length + (size_t)x * fb->var.bits_per_pixel / 8;

    if (fb->var.bits_per_pixel == 32) {
        uint32_t v = ((uint32_t)r << fb->var.red.offset) |
                     ((uint32_t)g << fb->var.green.offset) |
                     ((uint32_t)b << fb->var.blue.offset);
        memcpy(p, &v, sizeof(v));
    } else if (fb->var.bits_per_pixel == 16) {
        uint16_t v = (uint16_t)(((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3));
        memcpy(p, &v, sizeof(v));
    }
}

static int fb_open(struct framebuffer *fb, const char *path) {
    memset(fb, 0, sizeof(*fb));
    fb->fd = open(path, O_RDWR);
    if (fb->fd < 0) {
        perror(path);
        return -1;
    }
    if (ioctl(fb->fd, FBIOGET_FSCREENINFO, &fb->fix) < 0 ||
        ioctl(fb->fd, FBIOGET_VSCREENINFO, &fb->var) < 0) {
        perror("FBIOGET");
        close(fb->fd);
        return -1;
    }
    fb->size = (size_t)fb->fix.line_length * fb->var.yres_virtual;
    fb->mem = mmap(NULL, fb->size, PROT_READ | PROT_WRITE, MAP_SHARED, fb->fd, 0);
    if (fb->mem == MAP_FAILED) {
        perror("mmap");
        close(fb->fd);
        return -1;
    }
    return 0;
}

static void fb_close(struct framebuffer *fb) {
    if (fb->mem && fb->mem != MAP_FAILED) {
        munmap(fb->mem, fb->size);
    }
    if (fb->fd >= 0) {
        close(fb->fd);
    }
}

static void draw_solid(struct framebuffer *fb, uint32_t color) {
    for (int y = 0; y < (int)fb->var.yres; ++y) {
        for (int x = 0; x < (int)fb->var.xres; ++x) {
            put_pixel(fb, x, y, color);
        }
    }
}

static void draw_split(struct framebuffer *fb) {
    uint32_t colors[] = {rgb888(220, 40, 40), rgb888(30, 160, 80), rgb888(40, 100, 220), rgb888(245, 190, 40)};
    int mid_x = (int)fb->var.xres / 2;
    int mid_y = (int)fb->var.yres / 2;
    for (int y = 0; y < (int)fb->var.yres; ++y) {
        for (int x = 0; x < (int)fb->var.xres; ++x) {
            int idx = (x >= mid_x) + 2 * (y >= mid_y);
            put_pixel(fb, x, y, colors[idx]);
        }
    }
}

static void draw_gradient(struct framebuffer *fb) {
    int w = (int)fb->var.xres;
    int h = (int)fb->var.yres;
    for (int y = 0; y < h; ++y) {
        for (int x = 0; x < w; ++x) {
            uint8_t r = (uint8_t)(x * 255 / (w > 1 ? w - 1 : 1));
            uint8_t g = (uint8_t)(y * 255 / (h > 1 ? h - 1 : 1));
            uint8_t b = (uint8_t)((x + y) * 255 / (w + h > 2 ? w + h - 2 : 1));
            put_pixel(fb, x, y, rgb888(r, g, b));
        }
    }
}

static uint16_t rd16(const uint8_t *p) {
    return (uint16_t)p[0] | ((uint16_t)p[1] << 8);
}

static uint32_t rd32(const uint8_t *p) {
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8) | ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}

static int draw_bmp(struct framebuffer *fb, const char *path) {
    FILE *fp = fopen(path, "rb");
    if (!fp) {
        perror(path);
        return -1;
    }
    uint8_t hdr[54];
    if (fread(hdr, 1, sizeof(hdr), fp) != sizeof(hdr) || hdr[0] != 'B' || hdr[1] != 'M') {
        fprintf(stderr, "unsupported bmp header\n");
        fclose(fp);
        return -1;
    }
    uint32_t off = rd32(hdr + 10);
    int width = (int)rd32(hdr + 18);
    int height = (int)rd32(hdr + 22);
    uint16_t bpp = rd16(hdr + 28);
    if (width <= 0 || height == 0 || (bpp != 24 && bpp != 32)) {
        fprintf(stderr, "only uncompressed 24/32-bit bmp is supported\n");
        fclose(fp);
        return -1;
    }

    int top_down = height < 0;
    if (height < 0) {
        height = -height;
    }
    int bytes = bpp / 8;
    int stride = ((width * bytes + 3) / 4) * 4;
    uint8_t *row = malloc((size_t)stride);
    if (!row) {
        fclose(fp);
        return -1;
    }
    fseek(fp, (long)off, SEEK_SET);
    for (int row_idx = 0; row_idx < height; ++row_idx) {
        if (fread(row, 1, (size_t)stride, fp) != (size_t)stride) {
            break;
        }
        int y = top_down ? row_idx : height - 1 - row_idx;
        if (y >= (int)fb->var.yres) {
            continue;
        }
        for (int x = 0; x < width && x < (int)fb->var.xres; ++x) {
            uint8_t *px = row + x * bytes;
            put_pixel(fb, x, y, rgb888(px[2], px[1], px[0]));
        }
    }
    free(row);
    fclose(fp);
    return 0;
}

static void usage(const char *argv0) {
    fprintf(stderr,
            "usage: %s [solid #RRGGBB|split|gradient|bmp file.bmp]\n"
            "default: gradient\n",
            argv0);
}

int main(int argc, char **argv) {
    if (argc > 1 && strcmp(argv[1], "-h") == 0) {
        usage(argv[0]);
        return 0;
    }

    struct framebuffer fb;
    if (fb_open(&fb, "/dev/fb0") != 0) {
        return 1;
    }

    printf("fb0: %ux%u %ubpp stride=%u\n", fb.var.xres, fb.var.yres, fb.var.bits_per_pixel, fb.fix.line_length);

    const char *mode = argc > 1 ? argv[1] : "gradient";
    if (strcmp(mode, "solid") == 0) {
        draw_solid(&fb, parse_color(argc > 2 ? argv[2] : "#0080ff"));
    } else if (strcmp(mode, "split") == 0) {
        draw_split(&fb);
    } else if (strcmp(mode, "gradient") == 0) {
        draw_gradient(&fb);
    } else if (strcmp(mode, "bmp") == 0 && argc > 2) {
        int rc = draw_bmp(&fb, argv[2]);
        fb_close(&fb);
        return rc == 0 ? 0 : 1;
    } else {
        usage(argv[0]);
        fb_close(&fb);
        return 2;
    }

    fb_close(&fb);
    return 0;
}
