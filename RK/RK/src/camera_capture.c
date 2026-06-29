#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <sys/select.h>
#include <sys/time.h>
#include <time.h>
#include <unistd.h>
#include <linux/videodev2.h>

struct buffer {
    void *start[VIDEO_MAX_PLANES];
    size_t length[VIDEO_MAX_PLANES];
    uint32_t plane_count;
};

static int xioctl(int fd, unsigned long request, void *arg) {
    int rc;
    do {
        rc = ioctl(fd, request, arg);
    } while (rc < 0 && errno == EINTR);
    return rc;
}

static void fourcc_to_text(uint32_t pixfmt, char out[5]) {
    out[0] = (char)(pixfmt & 0xff);
    out[1] = (char)((pixfmt >> 8) & 0xff);
    out[2] = (char)((pixfmt >> 16) & 0xff);
    out[3] = (char)((pixfmt >> 24) & 0xff);
    out[4] = '\0';
}

static const char *buf_type_name(enum v4l2_buf_type type) {
    return type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE ? "mplane" : "single";
}

static int enum_formats(int fd, enum v4l2_buf_type type) {
    int count = 0;
    printf("formats[%s]:", buf_type_name(type));
    for (uint32_t i = 0;; ++i) {
        struct v4l2_fmtdesc desc;
        memset(&desc, 0, sizeof(desc));
        desc.index = i;
        desc.type = type;
        if (xioctl(fd, VIDIOC_ENUM_FMT, &desc) < 0) {
            break;
        }
        char fcc[5];
        fourcc_to_text(desc.pixelformat, fcc);
        printf(" %s", fcc);
        ++count;
    }
    if (count == 0) {
        printf(" none");
    }
    printf("\n");
    return count;
}

static void print_current_format(int fd, enum v4l2_buf_type type) {
    struct v4l2_format fmt;
    memset(&fmt, 0, sizeof(fmt));
    fmt.type = type;
    if (xioctl(fd, VIDIOC_G_FMT, &fmt) < 0) {
        printf("current[%s]: unavailable errno=%d\n", buf_type_name(type), errno);
        return;
    }
    char fcc[5];
    if (type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE) {
        fourcc_to_text(fmt.fmt.pix_mp.pixelformat, fcc);
        printf("current[%s]: %ux%u %s planes=%u\n", buf_type_name(type),
               fmt.fmt.pix_mp.width, fmt.fmt.pix_mp.height, fcc, fmt.fmt.pix_mp.num_planes);
    } else {
        fourcc_to_text(fmt.fmt.pix.pixelformat, fcc);
        printf("current[%s]: %ux%u %s\n", buf_type_name(type),
               fmt.fmt.pix.width, fmt.fmt.pix.height, fcc);
    }
}

static int set_format(int fd, enum v4l2_buf_type type, uint32_t pixfmt, int width, int height, struct v4l2_format *fmt) {
    memset(fmt, 0, sizeof(*fmt));
    fmt->type = type;
    if (type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE) {
        fmt->fmt.pix_mp.width = (uint32_t)width;
        fmt->fmt.pix_mp.height = (uint32_t)height;
        fmt->fmt.pix_mp.pixelformat = pixfmt;
        fmt->fmt.pix_mp.field = V4L2_FIELD_ANY;
    } else {
        fmt->fmt.pix.width = (uint32_t)width;
        fmt->fmt.pix.height = (uint32_t)height;
        fmt->fmt.pix.pixelformat = pixfmt;
        fmt->fmt.pix.field = V4L2_FIELD_ANY;
    }
    return xioctl(fd, VIDIOC_S_FMT, fmt);
}

static void put_yuv_pixel(uint8_t *dst, int yv, int u, int v) {
    int c = yv - 16;
    int r = (298 * c + 409 * v + 128) >> 8;
    int g = (298 * c - 100 * u - 208 * v + 128) >> 8;
    int b = (298 * c + 516 * u + 128) >> 8;
    dst[0] = (uint8_t)(r < 0 ? 0 : r > 255 ? 255 : r);
    dst[1] = (uint8_t)(g < 0 ? 0 : g > 255 ? 255 : g);
    dst[2] = (uint8_t)(b < 0 ? 0 : b > 255 ? 255 : b);
}

static void yuyv_to_rgb(const uint8_t *src, uint8_t *dst, int width, int height, int stride) {
    for (int y = 0; y < height; ++y) {
        const uint8_t *row = src + (size_t)y * stride;
        for (int x = 0; x + 1 < width; x += 2) {
            const uint8_t *px = row + x * 2;
            int u = px[1] - 128;
            int v = px[3] - 128;
            put_yuv_pixel(dst + ((size_t)y * width + x) * 3, px[0], u, v);
            put_yuv_pixel(dst + ((size_t)y * width + x + 1) * 3, px[2], u, v);
        }
    }
}

static void nv12_planes_to_rgb(const uint8_t *yplane,
                               const uint8_t *uvplane,
                               uint8_t *dst,
                               int width,
                               int height,
                               int y_stride,
                               int uv_stride) {
    for (int y = 0; y < height; ++y) {
        const uint8_t *yrow = yplane + (size_t)y * y_stride;
        const uint8_t *uvrow = uvplane + (size_t)(y / 2) * uv_stride;
        for (int x = 0; x < width; ++x) {
            int uv_index = x & ~1;
            int u = uvrow[uv_index] - 128;
            int v = uvrow[uv_index + 1] - 128;
            put_yuv_pixel(dst + ((size_t)y * width + x) * 3, yrow[x], u, v);
        }
    }
}

static void nv12_to_rgb(const uint8_t *src, uint8_t *dst, int width, int height, int y_stride, int uv_stride) {
    nv12_planes_to_rgb(src, src + (size_t)y_stride * height, dst, width, height, y_stride, uv_stride);
}

static int save_ppm(const char *path, const uint8_t *rgb, int width, int height) {
    FILE *fp = fopen(path, "wb");
    if (!fp) {
        perror(path);
        return -1;
    }
    fprintf(fp, "P6\n%d %d\n255\n", width, height);
    size_t need = (size_t)width * height * 3;
    int ok = fwrite(rgb, 1, need, fp) == need;
    fclose(fp);
    return ok ? 0 : -1;
}

static void usage(const char *argv0) {
    fprintf(stderr, "usage: %s [--probe] [--timeout seconds] [/dev/video0] [output.ppm] [width height]\n", argv0);
}

int main(int argc, char **argv) {
    int probe_only = 0;
    int timeout_sec = 5;
    const char *dev = "/dev/video0";
    const char *out = "/userdata/vision/capture.ppm";
    int width = 640;
    int height = 480;
    int pos = 0;

    for (int i = 1; i < argc; ++i) {
        if (strcmp(argv[i], "-h") == 0 || strcmp(argv[i], "--help") == 0) {
            usage(argv[0]);
            return 0;
        }
        if (strcmp(argv[i], "--probe") == 0) {
            probe_only = 1;
            continue;
        }
        if (strcmp(argv[i], "--timeout") == 0 && i + 1 < argc) {
            timeout_sec = atoi(argv[++i]);
            if (timeout_sec < 1) {
                timeout_sec = 1;
            }
            continue;
        }
        if (pos == 0) {
            dev = argv[i];
        } else if (pos == 1) {
            out = argv[i];
        } else if (pos == 2) {
            width = atoi(argv[i]);
        } else if (pos == 3) {
            height = atoi(argv[i]);
        } else {
            usage(argv[0]);
            return 2;
        }
        ++pos;
    }

    int fd = open(dev, O_RDWR | O_NONBLOCK);
    if (fd < 0) {
        perror(dev);
        return 1;
    }

    struct v4l2_capability cap;
    if (xioctl(fd, VIDIOC_QUERYCAP, &cap) < 0) {
        perror("VIDIOC_QUERYCAP");
        close(fd);
        return 1;
    }
    printf("camera: %s driver=%s card=%s bus=%s caps=0x%08x device_caps=0x%08x\n",
           dev, cap.driver, cap.card, cap.bus_info, cap.capabilities, cap.device_caps);

    enum_formats(fd, V4L2_BUF_TYPE_VIDEO_CAPTURE);
    enum_formats(fd, V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE);
    print_current_format(fd, V4L2_BUF_TYPE_VIDEO_CAPTURE);
    print_current_format(fd, V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE);
    if (probe_only) {
        close(fd);
        return 0;
    }

    uint32_t caps = cap.device_caps ? cap.device_caps : cap.capabilities;
    enum v4l2_buf_type types[2] = {
        (caps & V4L2_CAP_VIDEO_CAPTURE_MPLANE) ? V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE : V4L2_BUF_TYPE_VIDEO_CAPTURE,
        (caps & V4L2_CAP_VIDEO_CAPTURE_MPLANE) ? V4L2_BUF_TYPE_VIDEO_CAPTURE : V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE
    };
    uint32_t pixfmts[] = {V4L2_PIX_FMT_NV12, V4L2_PIX_FMT_YUYV};
    struct v4l2_format fmt;
    enum v4l2_buf_type buf_type = types[0];
    int format_ok = 0;

    for (size_t ti = 0; ti < sizeof(types) / sizeof(types[0]) && !format_ok; ++ti) {
        for (size_t pi = 0; pi < sizeof(pixfmts) / sizeof(pixfmts[0]) && !format_ok; ++pi) {
            if (set_format(fd, types[ti], pixfmts[pi], width, height, &fmt) == 0) {
                buf_type = types[ti];
                format_ok = 1;
            }
        }
    }
    if (!format_ok) {
        fprintf(stderr, "VIDIOC_S_FMT NV12/YUYV failed for single and mplane capture types\n");
        close(fd);
        return 1;
    }
    uint32_t pixfmt = 0;
    uint32_t plane_count = 1;
    if (buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE) {
        width = (int)fmt.fmt.pix_mp.width;
        height = (int)fmt.fmt.pix_mp.height;
        pixfmt = fmt.fmt.pix_mp.pixelformat;
        plane_count = fmt.fmt.pix_mp.num_planes ? fmt.fmt.pix_mp.num_planes : 1;
    } else {
        width = (int)fmt.fmt.pix.width;
        height = (int)fmt.fmt.pix.height;
        pixfmt = fmt.fmt.pix.pixelformat;
    }
    int y_stride = width;
    int uv_stride = width;
    int yuyv_stride = width * 2;
    if (buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE) {
        uint32_t p0_stride = fmt.fmt.pix_mp.plane_fmt[0].bytesperline;
        if (pixfmt == V4L2_PIX_FMT_NV12) {
            y_stride = p0_stride ? (int)p0_stride : width;
            if (plane_count >= 2 && fmt.fmt.pix_mp.plane_fmt[1].bytesperline) {
                uv_stride = (int)fmt.fmt.pix_mp.plane_fmt[1].bytesperline;
            } else {
                uv_stride = y_stride;
            }
        } else {
            yuyv_stride = p0_stride ? (int)p0_stride : width * 2;
        }
    } else {
        uint32_t stride = fmt.fmt.pix.bytesperline;
        if (pixfmt == V4L2_PIX_FMT_NV12) {
            y_stride = stride ? (int)stride : width;
            uv_stride = y_stride;
        } else {
            yuyv_stride = stride ? (int)stride : width * 2;
        }
    }
    char fourcc[5];
    fourcc_to_text(pixfmt, fourcc);
    printf("format: %dx%d %s type=%s planes=%u y_stride=%d uv_stride=%d yuyv_stride=%d\n",
           width, height, fourcc,
           buf_type_name(buf_type),
           plane_count,
           y_stride,
           uv_stride,
           yuyv_stride);
    if (pixfmt != V4L2_PIX_FMT_YUYV && pixfmt != V4L2_PIX_FMT_NV12) {
        fprintf(stderr, "unsupported capture format %s, expected YUYV or NV12\n", fourcc);
        close(fd);
        return 2;
    }

    struct v4l2_requestbuffers req;
    memset(&req, 0, sizeof(req));
    req.count = 4;
    req.type = buf_type;
    req.memory = V4L2_MEMORY_MMAP;
    if (xioctl(fd, VIDIOC_REQBUFS, &req) < 0 || req.count < 2) {
        perror("VIDIOC_REQBUFS");
        close(fd);
        return 1;
    }

    struct buffer *buffers = calloc(req.count, sizeof(*buffers));
    if (!buffers) {
        close(fd);
        return 1;
    }

    for (uint32_t i = 0; i < req.count; ++i) {
        struct v4l2_buffer buf;
        struct v4l2_plane planes[VIDEO_MAX_PLANES];
        memset(planes, 0, sizeof(planes));
        memset(&buf, 0, sizeof(buf));
        buf.type = buf_type;
        buf.memory = V4L2_MEMORY_MMAP;
        buf.index = i;
        if (buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE) {
            buf.length = VIDEO_MAX_PLANES;
            buf.m.planes = planes;
        }
        if (xioctl(fd, VIDIOC_QUERYBUF, &buf) < 0) {
            perror("VIDIOC_QUERYBUF");
            close(fd);
            return 1;
        }
        buffers[i].plane_count = buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE ? buf.length : 1;
        if (buffers[i].plane_count > VIDEO_MAX_PLANES) {
            buffers[i].plane_count = VIDEO_MAX_PLANES;
        }
        for (uint32_t p = 0; p < buffers[i].plane_count; ++p) {
            size_t length = buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE ? planes[p].length : buf.length;
            unsigned long offset = buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE ? planes[p].m.mem_offset : buf.m.offset;
            buffers[i].length[p] = length;
            buffers[i].start[p] = mmap(NULL, length, PROT_READ | PROT_WRITE, MAP_SHARED, fd, offset);
            if (buffers[i].start[p] == MAP_FAILED) {
                perror("mmap");
                close(fd);
                return 1;
            }
        }
        if (xioctl(fd, VIDIOC_QBUF, &buf) < 0) {
            perror("VIDIOC_QBUF");
            close(fd);
            return 1;
        }
    }

    enum v4l2_buf_type type = buf_type;
    if (xioctl(fd, VIDIOC_STREAMON, &type) < 0) {
        perror("VIDIOC_STREAMON");
        close(fd);
        return 1;
    }

    struct v4l2_buffer frame;
    struct v4l2_plane frame_planes[VIDEO_MAX_PLANES];
    int got = 0;
    for (int tries = 0; tries < timeout_sec && !got; ++tries) {
        fd_set fds;
        struct timeval tv;
        FD_ZERO(&fds);
        FD_SET(fd, &fds);
        tv.tv_sec = 1;
        tv.tv_usec = 0;
        int rc = select(fd + 1, &fds, NULL, NULL, &tv);
        if (rc < 0) {
            if (errno == EINTR) {
                continue;
            }
            perror("select");
            break;
        }
        if (rc == 0) {
            continue;
        }
        memset(&frame, 0, sizeof(frame));
        memset(frame_planes, 0, sizeof(frame_planes));
        frame.type = buf_type;
        frame.memory = V4L2_MEMORY_MMAP;
        if (buf_type == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE) {
            frame.length = VIDEO_MAX_PLANES;
            frame.m.planes = frame_planes;
        }
        if (xioctl(fd, VIDIOC_DQBUF, &frame) == 0) {
            got = 1;
        } else if (errno != EAGAIN) {
            perror("VIDIOC_DQBUF");
            break;
        }
    }

    int exit_code = 1;
    if (got) {
        uint8_t *rgb = malloc((size_t)width * height * 3);
        if (rgb) {
            if (pixfmt == V4L2_PIX_FMT_NV12) {
                if (buffers[frame.index].plane_count >= 2) {
                    nv12_planes_to_rgb((const uint8_t *)buffers[frame.index].start[0],
                                       (const uint8_t *)buffers[frame.index].start[1],
                                       rgb, width, height, y_stride, uv_stride);
                } else {
                    nv12_to_rgb((const uint8_t *)buffers[frame.index].start[0], rgb, width, height, y_stride, uv_stride);
                }
            } else {
                yuyv_to_rgb((const uint8_t *)buffers[frame.index].start[0], rgb, width, height, yuyv_stride);
            }
            if (save_ppm(out, rgb, width, height) == 0) {
                printf("saved %s\n", out);
                exit_code = 0;
            }
            free(rgb);
        }
        xioctl(fd, VIDIOC_QBUF, &frame);
    } else {
        fprintf(stderr, "failed to capture frame\n");
    }

    xioctl(fd, VIDIOC_STREAMOFF, &type);
    for (uint32_t i = 0; i < req.count; ++i) {
        for (uint32_t p = 0; p < buffers[i].plane_count; ++p) {
            if (buffers[i].start[p] && buffers[i].start[p] != MAP_FAILED) {
                munmap(buffers[i].start[p], buffers[i].length[p]);
            }
        }
    }
    free(buffers);
    close(fd);
    return exit_code;
}
