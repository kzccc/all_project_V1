#include <algorithm>
#include <cerrno>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <filesystem>
#include <linux/fb.h>
#include <linux/videodev2.h>
#include <drm/drm.h>
#include <drm/drm_mode.h>
#include <poll.h>
#include <string>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <sys/select.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include <vector>

namespace fs = std::filesystem;

static int clamp_int(int v, int lo, int hi) {
  return std::max(lo, std::min(v, hi));
}

static uint16_t rgb565(uint8_t r, uint8_t g, uint8_t b) {
  return static_cast<uint16_t>(((r & 0xf8) << 8) | ((g & 0xfc) << 3) | (b >> 3));
}

class FbDisplay {
public:
  bool open_dev(const char *path = "/dev/fb0") {
    const char *force_fb = std::getenv("FORCE_FBDEV");
    if ((!force_fb || std::strcmp(force_fb, "1") != 0) && open_drm()) return true;
    fd_ = ::open(path, O_RDWR);
    if (fd_ < 0) return false;
    if (ioctl(fd_, FBIOGET_FSCREENINFO, &finfo_) < 0 ||
        ioctl(fd_, FBIOGET_VSCREENINFO, &vinfo_) < 0) {
      close();
      return false;
    }
    width_ = static_cast<int>(vinfo_.xres);
    height_ = static_cast<int>(vinfo_.yres);
    bpp_ = static_cast<int>(vinfo_.bits_per_pixel);
    line_ = static_cast<int>(finfo_.line_length);
    size_ = static_cast<size_t>(line_) * vinfo_.yres_virtual;
    page_size_ = static_cast<size_t>(line_) * height_;
    mem_ = static_cast<uint8_t *>(mmap(nullptr, size_, PROT_READ | PROT_WRITE, MAP_SHARED, fd_, 0));
    if (mem_ == MAP_FAILED) {
      mem_ = nullptr;
      close();
      return false;
    }
    use_fb_pages_ = vinfo_.yres_virtual >= vinfo_.yres * 2 && page_size_ * 2 <= size_;
    if (use_fb_pages_) {
      front_page_ = 0;
      back_page_ = 1;
      draw_mem_ = mem_ + back_page_ * page_size_;
      std::memset(mem_, 0, size_);
      vinfo_.xoffset = 0;
      vinfo_.yoffset = 0;
      vinfo_.activate = FB_ACTIVATE_NOW;
      ioctl(fd_, FBIOPAN_DISPLAY, &vinfo_);
    } else {
      shadow_.assign(size_, 0);
      draw_mem_ = shadow_.data();
    }
    ioctl(fd_, FBIOBLANK, FB_BLANK_UNBLANK);
    return true;
  }

  void close() {
    if (drm_mode_) {
      if (saved_crtc_valid_) {
        uint64_t conn_ptr = connector_id_;
        saved_crtc_.set_connectors_ptr = reinterpret_cast<uint64_t>(&conn_ptr);
        saved_crtc_.count_connectors = 1;
        ioctl(fd_, DRM_IOCTL_MODE_SETCRTC, &saved_crtc_);
      }
      if (fb_id_) ioctl(fd_, DRM_IOCTL_MODE_RMFB, &fb_id_);
      if (mem_) munmap(mem_, size_);
      if (dumb_handle_) {
        drm_mode_destroy_dumb destroy{};
        destroy.handle = dumb_handle_;
        ioctl(fd_, DRM_IOCTL_MODE_DESTROY_DUMB, &destroy);
      }
      if (fd_ >= 0) ::close(fd_);
      fd_ = -1;
      mem_ = nullptr;
      drm_mode_ = false;
      shadow_.clear();
      draw_mem_ = nullptr;
      return;
    }
    if (mem_) munmap(mem_, size_);
    if (fd_ >= 0) ::close(fd_);
    fd_ = -1;
    mem_ = nullptr;
    shadow_.clear();
    draw_mem_ = nullptr;
  }

  ~FbDisplay() { close(); }

  int width() const { return width_; }
  int height() const { return height_; }

  void flush() {
    if (!mem_) return;
    if (use_fb_pages_) {
      vinfo_.yoffset = back_page_ * height_;
      vinfo_.activate = FB_ACTIVATE_VBL;
      if (ioctl(fd_, FBIOPAN_DISPLAY, &vinfo_) == 0) {
        front_page_ = back_page_;
        back_page_ = 1 - back_page_;
        draw_mem_ = mem_ + back_page_ * page_size_;
      }
      return;
    }
    if (shadow_.empty()) return;
    std::memcpy(mem_, shadow_.data(), std::min(size_, shadow_.size()));
    msync(mem_, size_, MS_SYNC);
  }

  void clear(uint32_t color) {
    for (int y = 0; y < height_; ++y) {
      for (int x = 0; x < width_; ++x) put_pixel(x, y, color);
    }
  }

  void put_pixel(int x, int y, uint32_t color) {
    if (!draw_mem_ || x < 0 || y < 0 || x >= width_ || y >= height_) return;
    uint8_t r = static_cast<uint8_t>((color >> 16) & 0xff);
    uint8_t g = static_cast<uint8_t>((color >> 8) & 0xff);
    uint8_t b = static_cast<uint8_t>(color & 0xff);
    uint8_t *p = draw_mem_ + y * line_ + x * (bpp_ / 8);
    if (bpp_ == 16) {
      *reinterpret_cast<uint16_t *>(p) = rgb565(r, g, b);
    } else if (bpp_ == 24) {
      p[0] = b;
      p[1] = g;
      p[2] = r;
    } else if (bpp_ == 32) {
      p[0] = b;
      p[1] = g;
      p[2] = r;
      p[3] = 0xff;
    }
  }

  void draw_rgb(int dx, int dy, int dw, int dh, const std::vector<uint8_t> &rgb, int sw, int sh) {
    if (rgb.empty() || sw <= 0 || sh <= 0) return;
    for (int y = 0; y < dh; ++y) {
      int sy = y * sh / dh;
      for (int x = 0; x < dw; ++x) {
        int sx = x * sw / dw;
        size_t idx = (static_cast<size_t>(sy) * sw + sx) * 3;
        put_pixel(dx + x, dy + y,
                  (static_cast<uint32_t>(rgb[idx]) << 16) |
                  (static_cast<uint32_t>(rgb[idx + 1]) << 8) |
                  static_cast<uint32_t>(rgb[idx + 2]));
      }
    }
  }

private:
  int fd_{-1};
  fb_fix_screeninfo finfo_{};
  fb_var_screeninfo vinfo_{};
  uint8_t *mem_{nullptr};
  uint8_t *draw_mem_{nullptr};
  size_t size_{0};
  size_t page_size_{0};
  int width_{0};
  int height_{0};
  int bpp_{0};
  int line_{0};
  std::vector<uint8_t> shadow_;
  bool use_fb_pages_{false};
  int front_page_{0};
  int back_page_{0};
  bool drm_mode_{false};
  uint32_t connector_id_{0};
  uint32_t crtc_id_{0};
  uint32_t fb_id_{0};
  uint32_t dumb_handle_{0};
  drm_mode_crtc saved_crtc_{};
  bool saved_crtc_valid_{false};

  bool open_drm() {
    fd_ = ::open("/dev/dri/card0", O_RDWR | O_CLOEXEC);
    if (fd_ < 0) return false;

    drm_mode_card_res res{};
    if (ioctl(fd_, DRM_IOCTL_MODE_GETRESOURCES, &res) < 0 || res.count_connectors == 0 || res.count_crtcs == 0) {
      ::close(fd_);
      fd_ = -1;
      return false;
    }
    std::vector<uint32_t> connectors(res.count_connectors);
    std::vector<uint32_t> crtcs(res.count_crtcs);
    std::vector<uint32_t> encs(res.count_encoders);
    std::vector<uint32_t> fbs(res.count_fbs);
    res.fb_id_ptr = reinterpret_cast<uint64_t>(fbs.data());
    res.connector_id_ptr = reinterpret_cast<uint64_t>(connectors.data());
    res.crtc_id_ptr = reinterpret_cast<uint64_t>(crtcs.data());
    res.encoder_id_ptr = reinterpret_cast<uint64_t>(encs.data());
    if (ioctl(fd_, DRM_IOCTL_MODE_GETRESOURCES, &res) < 0) {
      ::close(fd_);
      fd_ = -1;
      return false;
    }

    drm_mode_modeinfo mode{};
    uint32_t encoder_id = 0;
    for (uint32_t conn_id : connectors) {
      drm_mode_get_connector conn{};
      conn.connector_id = conn_id;
      if (ioctl(fd_, DRM_IOCTL_MODE_GETCONNECTOR, &conn) < 0) continue;
      if (conn.connection != 1 || conn.count_modes == 0 || conn.connector_type != DRM_MODE_CONNECTOR_DSI) continue;
      std::vector<drm_mode_modeinfo> modes(conn.count_modes);
      std::vector<uint32_t> conn_encoders(conn.count_encoders);
      std::vector<uint32_t> props(conn.count_props);
      std::vector<uint64_t> prop_values(conn.count_props);
      conn.modes_ptr = reinterpret_cast<uint64_t>(modes.data());
      conn.encoders_ptr = reinterpret_cast<uint64_t>(conn_encoders.data());
      conn.props_ptr = reinterpret_cast<uint64_t>(props.data());
      conn.prop_values_ptr = reinterpret_cast<uint64_t>(prop_values.data());
      if (ioctl(fd_, DRM_IOCTL_MODE_GETCONNECTOR, &conn) < 0) continue;
      connector_id_ = conn.connector_id;
      encoder_id = conn.encoder_id ? conn.encoder_id : (conn_encoders.empty() ? 0 : conn_encoders[0]);
      mode = modes[0];
      break;
    }
    if (!connector_id_ || !encoder_id) {
      ::close(fd_);
      fd_ = -1;
      return false;
    }

    drm_mode_get_encoder enc{};
    enc.encoder_id = encoder_id;
    if (ioctl(fd_, DRM_IOCTL_MODE_GETENCODER, &enc) < 0) {
      ::close(fd_);
      fd_ = -1;
      return false;
    }
    crtc_id_ = enc.crtc_id ? enc.crtc_id : (crtcs.empty() ? 0 : crtcs[0]);
    saved_crtc_.crtc_id = crtc_id_;
    saved_crtc_valid_ = ioctl(fd_, DRM_IOCTL_MODE_GETCRTC, &saved_crtc_) == 0;

    width_ = mode.hdisplay;
    height_ = mode.vdisplay;
    bpp_ = 32;

    drm_mode_create_dumb create{};
    create.width = width_;
    create.height = height_;
    create.bpp = 32;
    if (ioctl(fd_, DRM_IOCTL_MODE_CREATE_DUMB, &create) < 0) {
      ::close(fd_);
      fd_ = -1;
      return false;
    }
    dumb_handle_ = create.handle;
    line_ = create.pitch;
    size_ = create.size;

    drm_mode_fb_cmd fb{};
    fb.width = width_;
    fb.height = height_;
    fb.pitch = line_;
    fb.bpp = 32;
    fb.depth = 24;
    fb.handle = dumb_handle_;
    if (ioctl(fd_, DRM_IOCTL_MODE_ADDFB, &fb) < 0) {
      close();
      return false;
    }
    fb_id_ = fb.fb_id;

    drm_mode_map_dumb map{};
    map.handle = dumb_handle_;
    if (ioctl(fd_, DRM_IOCTL_MODE_MAP_DUMB, &map) < 0) {
      close();
      return false;
    }
    mem_ = static_cast<uint8_t *>(mmap(nullptr, size_, PROT_READ | PROT_WRITE, MAP_SHARED, fd_, map.offset));
    if (mem_ == MAP_FAILED) {
      mem_ = nullptr;
      close();
      return false;
    }
    std::memset(mem_, 0, size_);
    shadow_.assign(size_, 0);
    draw_mem_ = shadow_.data();

    drm_mode_crtc set{};
    uint64_t conn_ptr = connector_id_;
    set.crtc_id = crtc_id_;
    set.fb_id = fb_id_;
    set.set_connectors_ptr = reinterpret_cast<uint64_t>(&conn_ptr);
    set.count_connectors = 1;
    set.mode = mode;
    set.mode_valid = 1;
    if (ioctl(fd_, DRM_IOCTL_MODE_SETCRTC, &set) < 0) {
      close();
      return false;
    }
    drm_mode_ = true;
    return true;
  }
};

struct Frame {
  std::vector<uint8_t> bytes;
  std::vector<uint8_t> rgb;
  uint32_t pixfmt{};
  int width{};
  int height{};
};

class Camera {
public:
  bool open_auto() {
    if (const char *forced = std::getenv("CAMERA_DEV")) {
      if (open_dev(forced)) return true;
    }
    for (int i = 0; i < 16; ++i) {
      std::string path = "/dev/video" + std::to_string(i);
      if (!fs::exists(path)) continue;
      if (open_dev(path)) return true;
    }
    return false;
  }

  bool open_dev(const std::string &path) {
    close();
    fd_ = ::open(path.c_str(), O_RDWR | O_NONBLOCK);
    if (fd_ < 0) return false;
    v4l2_capability cap{};
    if (ioctl(fd_, VIDIOC_QUERYCAP, &cap) < 0) {
      close();
      return false;
    }
    uint32_t caps = cap.device_caps ? cap.device_caps : cap.capabilities;
    if (caps & V4L2_CAP_VIDEO_CAPTURE_MPLANE) type_ = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    else if (caps & V4L2_CAP_VIDEO_CAPTURE) type_ = V4L2_BUF_TYPE_VIDEO_CAPTURE;
    else {
      close();
      return false;
    }
    if (!set_format(V4L2_PIX_FMT_YUYV) && !set_format(V4L2_PIX_FMT_UYVY) &&
        !set_format(V4L2_PIX_FMT_NV12)) {
      close();
      return false;
    }
    if (!init_mmap() || !start()) {
      close();
      return false;
    }
    return true;
  }

  void close() {
    if (fd_ >= 0 && streaming_) {
      v4l2_buf_type type = type_;
      ioctl(fd_, VIDIOC_STREAMOFF, &type);
    }
    streaming_ = false;
    for (auto &b : bufs_) {
      if (b.start && b.start != MAP_FAILED) munmap(b.start, b.length);
    }
    bufs_.clear();
    if (fd_ >= 0) ::close(fd_);
    fd_ = -1;
  }

  ~Camera() { close(); }

  bool capture(Frame &frame, int timeout_ms = 2000) {
    if (fd_ < 0) return false;
    pollfd pfd{fd_, POLLIN, 0};
    if (::poll(&pfd, 1, timeout_ms) <= 0) return false;
    v4l2_buffer buf{};
    v4l2_plane planes[VIDEO_MAX_PLANES]{};
    buf.type = type_;
    buf.memory = V4L2_MEMORY_MMAP;
    if (is_mplane()) {
      buf.m.planes = planes;
      buf.length = VIDEO_MAX_PLANES;
    }
    if (ioctl(fd_, VIDIOC_DQBUF, &buf) < 0) return false;
    frame.width = width_;
    frame.height = height_;
    size_t used = is_mplane() ? buf.m.planes[0].bytesused : buf.bytesused;
    frame.bytes.assign(static_cast<uint8_t *>(bufs_[buf.index].start),
                       static_cast<uint8_t *>(bufs_[buf.index].start) + used);
    if (pixfmt_ == V4L2_PIX_FMT_YUYV) yuyv_to_rgb(frame.bytes.data(), width_, height_, frame.rgb);
    else if (pixfmt_ == V4L2_PIX_FMT_UYVY) uyvy_to_rgb(frame.bytes.data(), width_, height_, frame.rgb);
    else if (pixfmt_ == V4L2_PIX_FMT_NV12) nv12_to_rgb(frame.bytes.data(), width_, height_, frame.rgb);
    ioctl(fd_, VIDIOC_QBUF, &buf);
    return true;
  }

private:
  struct Buffer {
    void *start{};
    size_t length{};
  };

  int fd_{-1};
  int width_{0};
  int height_{0};
  uint32_t pixfmt_{};
  v4l2_buf_type type_{V4L2_BUF_TYPE_VIDEO_CAPTURE};
  bool streaming_{false};
  std::vector<Buffer> bufs_;

  bool is_mplane() const { return type_ == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE; }

  bool set_format(uint32_t fmt) {
    v4l2_format f{};
    f.type = type_;
    if (is_mplane()) {
      f.fmt.pix_mp.width = 640;
      f.fmt.pix_mp.height = 480;
      f.fmt.pix_mp.pixelformat = fmt;
      f.fmt.pix_mp.field = V4L2_FIELD_NONE;
    } else {
      f.fmt.pix.width = 640;
      f.fmt.pix.height = 480;
      f.fmt.pix.pixelformat = fmt;
      f.fmt.pix.field = V4L2_FIELD_NONE;
    }
    if (ioctl(fd_, VIDIOC_S_FMT, &f) < 0) return false;
    if (is_mplane()) {
      width_ = static_cast<int>(f.fmt.pix_mp.width);
      height_ = static_cast<int>(f.fmt.pix_mp.height);
      pixfmt_ = f.fmt.pix_mp.pixelformat;
    } else {
      width_ = static_cast<int>(f.fmt.pix.width);
      height_ = static_cast<int>(f.fmt.pix.height);
      pixfmt_ = f.fmt.pix.pixelformat;
    }
    return pixfmt_ == fmt;
  }

  bool init_mmap() {
    v4l2_requestbuffers req{};
    req.count = 4;
    req.type = type_;
    req.memory = V4L2_MEMORY_MMAP;
    if (ioctl(fd_, VIDIOC_REQBUFS, &req) < 0 || req.count < 2) return false;
    bufs_.resize(req.count);
    for (uint32_t i = 0; i < req.count; ++i) {
      v4l2_buffer buf{};
      v4l2_plane planes[VIDEO_MAX_PLANES]{};
      buf.type = type_;
      buf.memory = V4L2_MEMORY_MMAP;
      buf.index = i;
      if (is_mplane()) {
        buf.m.planes = planes;
        buf.length = VIDEO_MAX_PLANES;
      }
      if (ioctl(fd_, VIDIOC_QUERYBUF, &buf) < 0) return false;
      bufs_[i].length = is_mplane() ? buf.m.planes[0].length : buf.length;
      off_t offset = is_mplane() ? buf.m.planes[0].m.mem_offset : buf.m.offset;
      bufs_[i].start = mmap(nullptr, bufs_[i].length, PROT_READ | PROT_WRITE, MAP_SHARED, fd_, offset);
      if (bufs_[i].start == MAP_FAILED) return false;
      if (ioctl(fd_, VIDIOC_QBUF, &buf) < 0) return false;
    }
    return true;
  }

  bool start() {
    v4l2_buf_type type = type_;
    if (ioctl(fd_, VIDIOC_STREAMON, &type) < 0) return false;
    streaming_ = true;
    return true;
  }

  static int yuv_to_rgb_component(int c) { return clamp_int(c, 0, 255); }

  static void put_yuv_pixel(int y, int u, int v, std::vector<uint8_t> &rgb, int &j) {
    u -= 128;
    v -= 128;
    int r = y + static_cast<int>(1.402 * v);
    int g = y - static_cast<int>(0.344136 * u + 0.714136 * v);
    int b = y + static_cast<int>(1.772 * u);
    rgb[j++] = static_cast<uint8_t>(yuv_to_rgb_component(r));
    rgb[j++] = static_cast<uint8_t>(yuv_to_rgb_component(g));
    rgb[j++] = static_cast<uint8_t>(yuv_to_rgb_component(b));
  }

  static void put_yuv_pair(int y0, int y1, int u, int v, std::vector<uint8_t> &rgb, int &j) {
    put_yuv_pixel(y0, u, v, rgb, j);
    put_yuv_pixel(y1, u, v, rgb, j);
  }

  static void yuyv_to_rgb(const uint8_t *yuyv, int w, int h, std::vector<uint8_t> &rgb) {
    rgb.resize(static_cast<size_t>(w) * h * 3);
    for (int i = 0, j = 0; i < w * h * 2; i += 4) {
      put_yuv_pair(yuyv[i + 0], yuyv[i + 2], yuyv[i + 1], yuyv[i + 3], rgb, j);
    }
  }

  static void uyvy_to_rgb(const uint8_t *uyvy, int w, int h, std::vector<uint8_t> &rgb) {
    rgb.resize(static_cast<size_t>(w) * h * 3);
    for (int i = 0, j = 0; i < w * h * 2; i += 4) {
      put_yuv_pair(uyvy[i + 1], uyvy[i + 3], uyvy[i + 0], uyvy[i + 2], rgb, j);
    }
  }

  static void nv12_to_rgb(const uint8_t *nv12, int w, int h, std::vector<uint8_t> &rgb) {
    rgb.resize(static_cast<size_t>(w) * h * 3);
    const uint8_t *y_plane = nv12;
    const uint8_t *uv_plane = nv12 + static_cast<size_t>(w) * h;
    int j = 0;
    for (int y = 0; y < h; ++y) {
      for (int x = 0; x < w; ++x) {
        int yy = y_plane[y * w + x];
        int uv_index = (y / 2) * w + (x & ~1);
        put_yuv_pixel(yy, uv_plane[uv_index], uv_plane[uv_index + 1], rgb, j);
      }
    }
  }
};

int main() {
  FbDisplay fb;
  if (!fb.open_dev()) return 1;
  Camera cam;
  if (!cam.open_auto()) return 2;

  bool running = true;
  while (running) {
    Frame frame;
    if (!cam.capture(frame, 1000) || frame.rgb.empty()) continue;
    fb.clear(0x000000);
    fb.draw_rgb(0, 0, fb.width(), fb.height(), frame.rgb, frame.width, frame.height);
    fb.flush();

    char key = 0;
    timeval tv{0, 0};
    fd_set rfds;
    FD_ZERO(&rfds);
    FD_SET(STDIN_FILENO, &rfds);
    if (select(STDIN_FILENO + 1, &rfds, nullptr, nullptr, &tv) > 0) {
      ssize_t n = ::read(STDIN_FILENO, &key, 1);
      if (n > 0 && (key == 'q' || key == 'Q')) running = false;
    }
  }

  return 0;
}
