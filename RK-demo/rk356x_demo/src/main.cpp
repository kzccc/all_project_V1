#include <algorithm>
#include <array>
#include <cerrno>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <filesystem>
#include <linux/fb.h>
#include <linux/input.h>
#include <linux/videodev2.h>
#include <drm/drm.h>
#include <drm/drm_mode.h>
#include <poll.h>
#include <cctype>
#include <ctime>
#include <sstream>
#include <string>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <sys/select.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include <vector>

#define STB_IMAGE_IMPLEMENTATION
#define STB_IMAGE_WRITE_IMPLEMENTATION
#include "../third_party/stb/stb_image.h"
#include "../third_party/stb/stb_image_write.h"

#include "rknn_api.h"

namespace fs = std::filesystem;

static constexpr uint32_t C_BG = 0x2a1e17;
static constexpr uint32_t C_LEATHER = 0x4b3528;
static constexpr uint32_t C_LEATHER_2 = 0x5f4332;
static constexpr uint32_t C_ACCENT = 0xd8a15d;
static constexpr uint32_t C_WARN = 0xe2c07a;
static constexpr uint32_t C_DANGER = 0xb96a63;
static constexpr uint32_t C_TEXT = 0xf2e8d8;
static constexpr uint32_t C_MUTED = 0xbda58e;
static constexpr uint32_t C_GOLD = 0xcfa25d;

static constexpr int YOLO_CLASS_NUM = 80;
static constexpr int YOLO_PROP_BOX_SIZE = 5 + YOLO_CLASS_NUM;
static constexpr int YOLO_MAX_BOXES = 64;
static constexpr float YOLO_BOX_THRESH = 0.35f;
static constexpr float YOLO_NMS_THRESH = 0.45f;

static std::string trim_copy(std::string s) {
  while (!s.empty() && std::isspace(static_cast<unsigned char>(s.front()))) s.erase(s.begin());
  while (!s.empty() && std::isspace(static_cast<unsigned char>(s.back()))) s.pop_back();
  return s;
}

static std::string clip_text(const std::string &s, size_t max_len) {
  if (s.size() <= max_len) return s;
  if (max_len <= 3) return s.substr(0, max_len);
  return s.substr(0, max_len - 3) + "...";
}

static int clamp_int(int v, int lo, int hi) {
  return std::max(lo, std::min(v, hi));
}

static uint32_t leather_noise(int x, int y) {
  int n = (x * 73856093) ^ (y * 19349663) ^ (x * y * 83492791);
  n ^= (n >> 13);
  n *= 1274126177;
  uint8_t v = static_cast<uint8_t>(160 + (n & 0x1f));
  uint8_t r = static_cast<uint8_t>(76 + (v - 160) / 3);
  uint8_t g = static_cast<uint8_t>(50 + (v - 160) / 4);
  uint8_t b = static_cast<uint8_t>(36 + (v - 160) / 6);
  return (static_cast<uint32_t>(r) << 16) | (static_cast<uint32_t>(g) << 8) | b;
}

static uint16_t rgb565(uint8_t r, uint8_t g, uint8_t b) {
  return static_cast<uint16_t>(((r & 0xf8) << 8) | ((g & 0xfc) << 3) | (b >> 3));
}

static void split_rgb(uint32_t color, uint8_t &r, uint8_t &g, uint8_t &b) {
  r = static_cast<uint8_t>((color >> 16) & 0xff);
  g = static_cast<uint8_t>((color >> 8) & 0xff);
  b = static_cast<uint8_t>(color & 0xff);
}

static uint32_t blend_rgb(uint32_t a, uint32_t b, float t) {
  t = std::max(0.0f, std::min(1.0f, t));
  uint8_t ar, ag, ab, br, bg, bb;
  split_rgb(a, ar, ag, ab);
  split_rgb(b, br, bg, bb);
  auto mix = [&](uint8_t x, uint8_t y) {
    return static_cast<uint8_t>(x * (1.0f - t) + y * t);
  };
  return (static_cast<uint32_t>(mix(ar, br)) << 16) |
         (static_cast<uint32_t>(mix(ag, bg)) << 8) |
         static_cast<uint32_t>(mix(ab, bb));
}

class FbDisplay {
public:
  bool open_dev(const char *path = "/dev/fb0") {
    if (open_drm()) return true;
    fd_ = ::open(path, O_RDWR);
    if (fd_ < 0) {
      std::perror("open framebuffer");
      return false;
    }
    if (ioctl(fd_, FBIOGET_FSCREENINFO, &finfo_) < 0 ||
        ioctl(fd_, FBIOGET_VSCREENINFO, &vinfo_) < 0) {
      std::perror("ioctl framebuffer");
      close();
      return false;
    }
    width_ = static_cast<int>(vinfo_.xres);
    height_ = static_cast<int>(vinfo_.yres);
    bpp_ = static_cast<int>(vinfo_.bits_per_pixel);
    line_ = static_cast<int>(finfo_.line_length);
    size_ = static_cast<size_t>(line_) * vinfo_.yres_virtual;
    page_size_ = static_cast<size_t>(line_) * height_;
    mem_ = static_cast<uint8_t *>(mmap(nullptr, size_, PROT_READ | PROT_WRITE,
                                       MAP_SHARED, fd_, 0));
    if (mem_ == MAP_FAILED) {
      mem_ = nullptr;
      std::perror("mmap framebuffer");
      close();
      return false;
    }
    vinfo_.xoffset = 0;
    vinfo_.yoffset = 0;
    vinfo_.activate = FB_ACTIVATE_NOW;
    ioctl(fd_, FBIOPAN_DISPLAY, &vinfo_);
    ioctl(fd_, FBIOBLANK, FB_BLANK_UNBLANK);
    use_fb_pages_ = vinfo_.yres_virtual >= vinfo_.yres * 2 && page_size_ * 2 <= size_;
    if (use_fb_pages_) {
      front_page_ = 0;
      back_page_ = 1;
      draw_mem_ = mem_ + back_page_ * page_size_;
      std::memset(mem_, 0, size_);
    } else {
      shadow_.assign(size_, 0);
      draw_mem_ = shadow_.data();
    }
    std::printf("LCD framebuffer: %dx%d %dbpp line=%d\n", width_, height_, bpp_, line_);
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
      if (mem_) {
        munmap(mem_, size_);
        mem_ = nullptr;
      }
      if (dumb_handle_) {
        drm_mode_destroy_dumb destroy{};
        destroy.handle = dumb_handle_;
        ioctl(fd_, DRM_IOCTL_MODE_DESTROY_DUMB, &destroy);
      }
      if (fd_ >= 0) {
        ::close(fd_);
        fd_ = -1;
      }
      drm_mode_ = false;
      shadow_.clear();
      draw_mem_ = nullptr;
      return;
    }
    if (mem_) {
      munmap(mem_, size_);
      mem_ = nullptr;
    }
    if (fd_ >= 0) {
      ::close(fd_);
      fd_ = -1;
    }
    shadow_.clear();
    draw_mem_ = nullptr;
  }

  ~FbDisplay() { close(); }

  int width() const { return width_; }
  int height() const { return height_; }
  bool valid() const { return mem_ != nullptr; }

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

  void clear(uint32_t color) { fill_rect(0, 0, width_, height_, color); }

  void put_pixel(int x, int y, uint32_t color) {
    if (!draw_mem_ || x < 0 || y < 0 || x >= width_ || y >= height_) return;
    uint8_t r, g, b;
    split_rgb(color, r, g, b);
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

  void fill_rect(int x, int y, int w, int h, uint32_t color) {
    int x0 = clamp_int(x, 0, width_);
    int y0 = clamp_int(y, 0, height_);
    int x1 = clamp_int(x + w, 0, width_);
    int y1 = clamp_int(y + h, 0, height_);
    for (int yy = y0; yy < y1; ++yy) {
      for (int xx = x0; xx < x1; ++xx) put_pixel(xx, yy, color);
    }
  }

  void draw_rect(int x, int y, int w, int h, uint32_t color) {
    fill_rect(x, y, w, 2, color);
    fill_rect(x, y + h - 2, w, 2, color);
    fill_rect(x, y, 2, h, color);
    fill_rect(x + w - 2, y, 2, h, color);
  }

  void draw_play_triangle(int cx, int cy, int size, uint32_t color) {
    int left = cx - size / 2;
    for (int yy = -size; yy <= size; ++yy) {
      int span = size - std::abs(yy);
      fill_rect(left, cy + yy, span + 1, 1, color);
    }
  }

  void draw_rgb(int dx, int dy, int dw, int dh, const std::vector<uint8_t> &rgb,
                int sw, int sh) {
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

  void draw_text(int x, int y, const std::string &text, uint32_t color, int scale = 2) {
    int cx = x;
    for (char ch : text) {
      if (ch == '\n') {
        cx = x;
        y += 8 * scale;
        continue;
      }
      draw_char(cx, y, ch, color, scale);
      cx += 6 * scale;
    }
  }

  bool draw_bmp(const std::string &path, int dx, int dy) {
    FILE *fp = std::fopen(path.c_str(), "rb");
    if (!fp) return false;
    uint8_t header[54]{};
    if (std::fread(header, 1, sizeof(header), fp) != sizeof(header) ||
        header[0] != 'B' || header[1] != 'M') {
      std::fclose(fp);
      return false;
    }
    uint32_t data_offset = *reinterpret_cast<uint32_t *>(&header[10]);
    int32_t w = *reinterpret_cast<int32_t *>(&header[18]);
    int32_t h_raw = *reinterpret_cast<int32_t *>(&header[22]);
    uint16_t bits = *reinterpret_cast<uint16_t *>(&header[28]);
    if (w <= 0 || h_raw == 0 || (bits != 24 && bits != 32)) {
      std::fclose(fp);
      return false;
    }
    int h = std::abs(h_raw);
    int row_stride = ((w * bits + 31) / 32) * 4;
    std::vector<uint8_t> row(row_stride);
    std::fseek(fp, data_offset, SEEK_SET);
    for (int row_i = 0; row_i < h; ++row_i) {
      if (std::fread(row.data(), 1, row.size(), fp) != row.size()) break;
      int y = h_raw > 0 ? (h - 1 - row_i) : row_i;
      for (int x = 0; x < w; ++x) {
        uint8_t *p = row.data() + x * (bits / 8);
        put_pixel(dx + x, dy + y, (static_cast<uint32_t>(p[2]) << 16) |
                                      (static_cast<uint32_t>(p[1]) << 8) | p[0]);
      }
    }
    std::fclose(fp);
    return true;
  }

private:
  void draw_char(int x, int y, char ch, uint32_t color, int scale) {
    const uint8_t *glyph = glyph_for(ch);
    for (int row = 0; row < 7; ++row) {
      for (int col = 0; col < 5; ++col) {
        if (glyph[row] & (1u << (4 - col))) {
          fill_rect(x + col * scale, y + row * scale, scale, scale, color);
        }
      }
    }
  }

  static const uint8_t *glyph_for(char c) {
    static const std::array<uint8_t, 7> blank{0, 0, 0, 0, 0, 0, 0};
    static const std::array<uint8_t, 7> unknown{0x1f, 0x11, 0x04, 0x04, 0x00, 0x04, 0x00};
    static const std::array<std::array<uint8_t, 7>, 96> font = make_font();
    unsigned char uc = static_cast<unsigned char>(c);
    if (uc < 32 || uc > 127) return blank.data();
    const auto &g = font[uc - 32];
    bool empty = std::all_of(g.begin(), g.end(), [](uint8_t v) { return v == 0; });
    return empty && c != ' ' ? unknown.data() : g.data();
  }

  static std::array<std::array<uint8_t, 7>, 96> make_font() {
    std::array<std::array<uint8_t, 7>, 96> f{};
    auto set = [&](char c, std::array<uint8_t, 7> g) { f[c - 32] = g; };
    set(' ', {0, 0, 0, 0, 0, 0, 0});
    set(':', {0, 4, 4, 0, 4, 4, 0});
    set('-', {0, 0, 0, 0x1f, 0, 0, 0});
    set('/', {1, 2, 4, 8, 0x10, 0, 0});
    set('.', {0, 0, 0, 0, 0, 0x0c, 0x0c});
    set('0', {0x0e, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0e});
    set('1', {0x04, 0x0c, 0x04, 0x04, 0x04, 0x04, 0x0e});
    set('2', {0x0e, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1f});
    set('3', {0x1f, 0x02, 0x04, 0x02, 0x01, 0x11, 0x0e});
    set('4', {0x02, 0x06, 0x0a, 0x12, 0x1f, 0x02, 0x02});
    set('5', {0x1f, 0x10, 0x1e, 0x01, 0x01, 0x11, 0x0e});
    set('6', {0x06, 0x08, 0x10, 0x1e, 0x11, 0x11, 0x0e});
    set('7', {0x1f, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08});
    set('8', {0x0e, 0x11, 0x11, 0x0e, 0x11, 0x11, 0x0e});
    set('9', {0x0e, 0x11, 0x11, 0x0f, 0x01, 0x02, 0x0c});
    set('A', {0x0e, 0x11, 0x11, 0x1f, 0x11, 0x11, 0x11});
    set('B', {0x1e, 0x11, 0x11, 0x1e, 0x11, 0x11, 0x1e});
    set('C', {0x0e, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0e});
    set('D', {0x1e, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1e});
    set('E', {0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x1f});
    set('F', {0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x10});
    set('G', {0x0e, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0f});
    set('H', {0x11, 0x11, 0x11, 0x1f, 0x11, 0x11, 0x11});
    set('I', {0x0e, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0e});
    set('J', {0x07, 0x02, 0x02, 0x02, 0x12, 0x12, 0x0c});
    set('K', {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11});
    set('L', {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1f});
    set('M', {0x11, 0x1b, 0x15, 0x15, 0x11, 0x11, 0x11});
    set('N', {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11});
    set('O', {0x0e, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0e});
    set('P', {0x1e, 0x11, 0x11, 0x1e, 0x10, 0x10, 0x10});
    set('Q', {0x0e, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0d});
    set('R', {0x1e, 0x11, 0x11, 0x1e, 0x14, 0x12, 0x11});
    set('S', {0x0f, 0x10, 0x10, 0x0e, 0x01, 0x01, 0x1e});
    set('T', {0x1f, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04});
    set('U', {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0e});
    set('V', {0x11, 0x11, 0x11, 0x11, 0x11, 0x0a, 0x04});
    set('W', {0x11, 0x11, 0x11, 0x15, 0x15, 0x1b, 0x11});
    set('X', {0x11, 0x11, 0x0a, 0x04, 0x0a, 0x11, 0x11});
    set('Y', {0x11, 0x11, 0x0a, 0x04, 0x04, 0x04, 0x04});
    set('Z', {0x1f, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1f});
    for (char c = 'a'; c <= 'z'; ++c) f[c - 32] = f[c - 'a' + 'A' - 32];
    return f;
  }

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
    if (fd_ < 0) {
      std::perror("open /dev/dri/card0");
      return false;
    }

    drm_mode_card_res res{};
    if (ioctl(fd_, DRM_IOCTL_MODE_GETRESOURCES, &res) < 0 || res.count_connectors == 0 || res.count_crtcs == 0) {
      std::perror("DRM_IOCTL_MODE_GETRESOURCES initial");
      std::fprintf(stderr, "DRM resources counts: connectors=%u crtcs=%u\n", res.count_connectors, res.count_crtcs);
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
      std::perror("DRM_IOCTL_MODE_GETRESOURCES ids");
      ::close(fd_);
      fd_ = -1;
      return false;
    }

    drm_mode_modeinfo mode{};
    uint32_t encoder_id = 0;
    for (uint32_t conn_id : connectors) {
      drm_mode_get_connector conn{};
      conn.connector_id = conn_id;
      if (ioctl(fd_, DRM_IOCTL_MODE_GETCONNECTOR, &conn) < 0) {
        std::perror("DRM_IOCTL_MODE_GETCONNECTOR probe");
        continue;
      }
      std::fprintf(stderr, "DRM connector id=%u type=%u connection=%u modes=%u encoders=%u props=%u encoder=%u\n",
                   conn.connector_id, conn.connector_type, conn.connection, conn.count_modes,
                   conn.count_encoders, conn.count_props, conn.encoder_id);
      if (conn.connection != 1 || conn.count_modes == 0) continue;
      if (conn.connector_type != DRM_MODE_CONNECTOR_DSI) continue;
      std::vector<drm_mode_modeinfo> modes(conn.count_modes);
      std::vector<uint32_t> encoders(conn.count_encoders);
      std::vector<uint32_t> props(conn.count_props);
      std::vector<uint64_t> prop_values(conn.count_props);
      conn.modes_ptr = reinterpret_cast<uint64_t>(modes.data());
      conn.encoders_ptr = reinterpret_cast<uint64_t>(encoders.data());
      conn.props_ptr = reinterpret_cast<uint64_t>(props.data());
      conn.prop_values_ptr = reinterpret_cast<uint64_t>(prop_values.data());
      if (ioctl(fd_, DRM_IOCTL_MODE_GETCONNECTOR, &conn) < 0) {
        std::perror("DRM_IOCTL_MODE_GETCONNECTOR full");
        continue;
      }
      connector_id_ = conn.connector_id;
      encoder_id = conn.encoder_id ? conn.encoder_id : (encoders.empty() ? 0 : encoders[0]);
      mode = modes[0];
      for (const auto &m : modes) {
        if (m.type & DRM_MODE_TYPE_PREFERRED) {
          mode = m;
          break;
        }
      }
      break;
    }
    if (!connector_id_ || !encoder_id) {
      std::fprintf(stderr, "DRM no connected DSI connector found connector=%u encoder=%u\n", connector_id_, encoder_id);
      ::close(fd_);
      fd_ = -1;
      return false;
    }

    drm_mode_get_encoder enc{};
    enc.encoder_id = encoder_id;
    if (ioctl(fd_, DRM_IOCTL_MODE_GETENCODER, &enc) < 0) {
      std::perror("DRM_IOCTL_MODE_GETENCODER");
      ::close(fd_);
      fd_ = -1;
      return false;
    }
    crtc_id_ = enc.crtc_id ? enc.crtc_id : (crtcs.empty() ? 0 : crtcs[0]);
    if (!crtc_id_) {
      std::fprintf(stderr, "DRM no crtc found encoder=%u possible=0x%x\n", encoder_id, enc.possible_crtcs);
      ::close(fd_);
      fd_ = -1;
      return false;
    }

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
      std::perror("DRM_IOCTL_MODE_CREATE_DUMB");
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
      std::perror("DRM_IOCTL_MODE_ADDFB");
      close();
      return false;
    }
    fb_id_ = fb.fb_id;

    drm_mode_map_dumb map{};
    map.handle = dumb_handle_;
    if (ioctl(fd_, DRM_IOCTL_MODE_MAP_DUMB, &map) < 0) {
      std::perror("DRM_IOCTL_MODE_MAP_DUMB");
      close();
      return false;
    }
    mem_ = static_cast<uint8_t *>(mmap(nullptr, size_, PROT_READ | PROT_WRITE, MAP_SHARED, fd_, map.offset));
    if (mem_ == MAP_FAILED) {
      std::perror("mmap drm dumb");
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
      std::perror("DRM_IOCTL_MODE_SETCRTC");
      close();
      return false;
    }

    drm_mode_ = true;
    std::printf("LCD DRM/KMS: %dx%d 32bpp line=%d connector=%u crtc=%u\n",
                width_, height_, line_, connector_id_, crtc_id_);
    return true;
  }
};

static void fill_leather_panel(FbDisplay &fb, int x, int y, int w, int h, uint32_t base,
                               uint32_t border) {
  fb.fill_rect(x, y, w, h, base);
  fb.draw_rect(x, y, w, h, border);
  fb.fill_rect(x + 2, y + 2, w - 4, 2, blend_rgb(base, C_TEXT, 0.08f));
  for (int i = 10; i + 10 < w; i += 42) {
    int sx = x + i;
    for (int j = 0; j < h - 8; j += 8) {
      fb.fill_rect(sx, y + 5 + j, 2, 2, blend_rgb(base, C_TEXT, 0.10f));
    }
  }
  for (int i = 12; i + 12 < h; i += 46) {
    fb.fill_rect(x + 6, y + i, w - 12, 1, blend_rgb(base, C_LEATHER_2, 0.18f));
  }
}

struct Button {
  std::string label;
  int x{};
  int y{};
  int w{};
  int h{};
  uint32_t color{};

  bool contains(int px, int py) const {
    return px >= x && px < x + w && py >= y && py < y + h;
  }
};

static bool test_bit(const unsigned long *bits, int bit) {
  return bits[bit / (8 * sizeof(unsigned long))] & (1UL << (bit % (8 * sizeof(unsigned long))));
}

class TouchInput {
public:
  bool open_auto(int screen_w, int screen_h) {
    screen_w_ = screen_w;
    screen_h_ = screen_h;
    for (const auto &entry : fs::directory_iterator("/dev/input")) {
      const std::string path = entry.path();
      if (path.find("event") == std::string::npos) continue;
      int fd = ::open(path.c_str(), O_RDONLY | O_NONBLOCK);
      if (fd < 0) continue;

      unsigned long ev_bits[4]{};
      unsigned long abs_bits[8]{};
      unsigned long key_bits[16]{};
      ioctl(fd, EVIOCGBIT(0, sizeof(ev_bits)), ev_bits);
      ioctl(fd, EVIOCGBIT(EV_ABS, sizeof(abs_bits)), abs_bits);
      ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(key_bits)), key_bits);

      bool has_abs = test_bit(ev_bits, EV_ABS);
      bool has_xy = test_bit(abs_bits, ABS_X) && test_bit(abs_bits, ABS_Y);
      bool has_mt = test_bit(abs_bits, ABS_MT_POSITION_X) && test_bit(abs_bits, ABS_MT_POSITION_Y);
      bool has_touch = test_bit(key_bits, BTN_TOUCH) || has_mt;
      if (has_abs && (has_xy || has_mt) && has_touch) {
        fd_ = fd;
        path_ = path;
        x_code_ = has_mt ? ABS_MT_POSITION_X : ABS_X;
        y_code_ = has_mt ? ABS_MT_POSITION_Y : ABS_Y;
        input_absinfo ax{}, ay{};
        ioctl(fd_, EVIOCGABS(x_code_), &ax);
        ioctl(fd_, EVIOCGABS(y_code_), &ay);
        x_min_ = ax.minimum;
        x_max_ = ax.maximum > ax.minimum ? ax.maximum : screen_w - 1;
        y_min_ = ay.minimum;
        y_max_ = ay.maximum > ay.minimum ? ay.maximum : screen_h - 1;
        std::printf("Touch input: %s x=[%d,%d] y=[%d,%d]\n", path_.c_str(), x_min_, x_max_, y_min_, y_max_);
        return true;
      }
      ::close(fd);
    }
    std::fprintf(stderr, "No touchscreen event device found.\n");
    return false;
  }

  ~TouchInput() {
    if (fd_ >= 0) ::close(fd_);
  }

  bool poll_tap(int timeout_ms, int &sx, int &sy, bool &is_swipe, int &ex, int &ey) {
    if (fd_ < 0) {
      usleep(timeout_ms * 1000);
      return false;
    }
    pollfd pfd{fd_, POLLIN, 0};
    int ret = ::poll(&pfd, 1, timeout_ms);
    if (ret <= 0) return false;

    input_event ev{};
    bool got_report = false;
    while (::read(fd_, &ev, sizeof(ev)) == sizeof(ev)) {
      if (ev.type == EV_ABS) {
        if (ev.code == x_code_) raw_x_ = ev.value;
        if (ev.code == y_code_) raw_y_ = ev.value;
      } else if (ev.type == EV_KEY && ev.code == BTN_TOUCH) {
        down_ = ev.value != 0;
        if (down_) {
          start_x_ = scale_x(raw_x_);
          start_y_ = scale_y(raw_y_);
        }
      } else if (ev.type == EV_SYN && ev.code == SYN_REPORT) {
        got_report = true;
      }
    }

    int cx = scale_x(raw_x_);
    int cy = scale_y(raw_y_);
    if (got_report && !down_ && was_down_) {
      sx = start_x_;
      sy = start_y_;
      ex = cx;
      ey = cy;
      is_swipe = std::abs(ex - sx) > 80 || std::abs(ey - sy) > 80;
      was_down_ = false;
      return true;
    }
    if (got_report && down_) {
      was_down_ = true;
    }
    return false;
  }

private:
  int scale_x(int raw) const {
    return clamp_int((raw - x_min_) * screen_w_ / std::max(1, x_max_ - x_min_), 0, screen_w_ - 1);
  }
  int scale_y(int raw) const {
    return clamp_int((raw - y_min_) * screen_h_ / std::max(1, y_max_ - y_min_), 0, screen_h_ - 1);
  }

  int fd_{-1};
  std::string path_;
  int screen_w_{1};
  int screen_h_{1};
  int x_code_{ABS_X};
  int y_code_{ABS_Y};
  int x_min_{0};
  int x_max_{1};
  int y_min_{0};
  int y_max_{1};
  int raw_x_{0};
  int raw_y_{0};
  bool down_{false};
  bool was_down_{false};
  int start_x_{0};
  int start_y_{0};
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
      std::fprintf(stderr, "Forced camera %s failed, falling back to auto scan.\n", forced);
    }
    if (fs::exists("/dev/v4l/by-id")) {
      for (const auto &entry : fs::directory_iterator("/dev/v4l/by-id")) {
        std::string path = entry.path();
        if (path.find("index0") != std::string::npos && open_dev(path)) return true;
      }
    }
    for (int i = 0; i < 16; ++i) {
      std::string path = "/dev/video" + std::to_string(i);
      if (!fs::exists(path)) continue;
      if (open_dev(path)) return true;
    }
    std::fprintf(stderr, "No usable V4L2 camera found.\n");
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
    if (caps & V4L2_CAP_VIDEO_CAPTURE_MPLANE) {
      type_ = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    } else if (caps & V4L2_CAP_VIDEO_CAPTURE) {
      type_ = V4L2_BUF_TYPE_VIDEO_CAPTURE;
    } else {
      close();
      return false;
    }
    path_ = path;
    if (!set_format(V4L2_PIX_FMT_YUYV) && !set_format(V4L2_PIX_FMT_UYVY) &&
        !set_format(V4L2_PIX_FMT_NV12) && !set_format(V4L2_PIX_FMT_MJPEG)) {
      std::fprintf(stderr, "%s: failed to set UYVY/YUYV/NV12/MJPEG format\n", path.c_str());
      close();
      return false;
    }
    if (!init_mmap() || !start()) {
      close();
      return false;
    }
    std::printf("Camera: %s %dx%d %s %s\n", path_.c_str(), width_, height_,
                fourcc(pixfmt_).c_str(),
                type_ == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE ? "mplane" : "single");
    return true;
  }

  void close() {
    stop();
    for (auto &b : bufs_) {
      if (b.start && b.start != MAP_FAILED) munmap(b.start, b.length);
    }
    bufs_.clear();
    if (fd_ >= 0) {
      ::close(fd_);
      fd_ = -1;
    }
  }

  ~Camera() { close(); }

  bool is_open() const { return fd_ >= 0; }

  bool capture(Frame &frame, int timeout_ms = 2000) {
    if (fd_ < 0) return false;
    pollfd pfd{fd_, POLLIN, 0};
    int ret = ::poll(&pfd, 1, timeout_ms);
    if (ret <= 0) {
      std::fprintf(stderr, "camera timeout\n");
      return false;
    }
    v4l2_buffer buf{};
    v4l2_plane planes[VIDEO_MAX_PLANES]{};
    buf.type = type_;
    buf.memory = V4L2_MEMORY_MMAP;
    if (is_mplane()) {
      buf.m.planes = planes;
      buf.length = VIDEO_MAX_PLANES;
    }
    if (ioctl(fd_, VIDIOC_DQBUF, &buf) < 0) {
      if (errno != EAGAIN) std::perror("VIDIOC_DQBUF");
      return false;
    }
    frame.width = width_;
    frame.height = height_;
    frame.pixfmt = pixfmt_;
    size_t used = is_mplane() ? buf.m.planes[0].bytesused : buf.bytesused;
    frame.bytes.assign(static_cast<uint8_t *>(bufs_[buf.index].start),
                       static_cast<uint8_t *>(bufs_[buf.index].start) + used);
    if (pixfmt_ == V4L2_PIX_FMT_YUYV) {
      yuyv_to_rgb(frame.bytes.data(), width_, height_, frame.rgb);
    } else if (pixfmt_ == V4L2_PIX_FMT_UYVY) {
      uyvy_to_rgb(frame.bytes.data(), width_, height_, frame.rgb);
    } else if (pixfmt_ == V4L2_PIX_FMT_NV12) {
      nv12_to_rgb(frame.bytes.data(), width_, height_, frame.rgb);
    }
    if (ioctl(fd_, VIDIOC_QBUF, &buf) < 0) std::perror("VIDIOC_QBUF");
    return true;
  }

private:
  struct Buffer {
    void *start{};
    size_t length{};
  };

  static std::string fourcc(uint32_t f) {
    char s[5]{static_cast<char>(f & 0xff), static_cast<char>((f >> 8) & 0xff),
              static_cast<char>((f >> 16) & 0xff), static_cast<char>((f >> 24) & 0xff), 0};
    return s;
  }

  bool set_format(uint32_t fmt) {
    v4l2_format f{};
    f.type = type_;
    if (is_mplane()) {
      f.fmt.pix_mp.width = 640;
      f.fmt.pix_mp.height = 480;
      f.fmt.pix_mp.pixelformat = fmt;
      f.fmt.pix_mp.field = V4L2_FIELD_ANY;
    } else {
      f.fmt.pix.width = 640;
      f.fmt.pix.height = 480;
      f.fmt.pix.pixelformat = fmt;
      f.fmt.pix.field = V4L2_FIELD_ANY;
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
    if (ioctl(fd_, VIDIOC_REQBUFS, &req) < 0 || req.count < 2) {
      std::perror("VIDIOC_REQBUFS");
      return false;
    }
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
      if (ioctl(fd_, VIDIOC_QUERYBUF, &buf) < 0) {
        std::perror("VIDIOC_QUERYBUF");
        return false;
      }
      bufs_[i].length = is_mplane() ? buf.m.planes[0].length : buf.length;
      off_t offset = is_mplane() ? buf.m.planes[0].m.mem_offset : buf.m.offset;
      bufs_[i].start = mmap(nullptr, bufs_[i].length, PROT_READ | PROT_WRITE, MAP_SHARED, fd_, offset);
      if (bufs_[i].start == MAP_FAILED) {
        std::perror("mmap camera");
        return false;
      }
      if (ioctl(fd_, VIDIOC_QBUF, &buf) < 0) {
        std::perror("VIDIOC_QBUF");
        return false;
      }
    }
    return true;
  }

  bool start() {
    if (streaming_) return true;
    v4l2_buf_type type = type_;
    if (ioctl(fd_, VIDIOC_STREAMON, &type) < 0) {
      std::perror("VIDIOC_STREAMON");
      return false;
    }
    streaming_ = true;
    return true;
  }

  void stop() {
    if (fd_ >= 0 && streaming_) {
      v4l2_buf_type type = type_;
      ioctl(fd_, VIDIOC_STREAMOFF, &type);
      streaming_ = false;
    }
  }

  bool is_mplane() const { return type_ == V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE; }

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
      int y0 = yuyv[i + 0];
      int u = yuyv[i + 1];
      int y1 = yuyv[i + 2];
      int v = yuyv[i + 3];
      put_yuv_pair(y0, y1, u, v, rgb, j);
    }
  }

  static void uyvy_to_rgb(const uint8_t *uyvy, int w, int h, std::vector<uint8_t> &rgb) {
    rgb.resize(static_cast<size_t>(w) * h * 3);
    for (int i = 0, j = 0; i < w * h * 2; i += 4) {
      int u = uyvy[i + 0];
      int y0 = uyvy[i + 1];
      int v = uyvy[i + 2];
      int y1 = uyvy[i + 3];
      put_yuv_pair(y0, y1, u, v, rgb, j);
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
        int u = uv_plane[uv_index];
        int v = uv_plane[uv_index + 1];
        put_yuv_pixel(yy, u, v, rgb, j);
      }
    }
  }

  int fd_{-1};
  std::string path_;
  int width_{0};
  int height_{0};
  uint32_t pixfmt_{};
  v4l2_buf_type type_{V4L2_BUF_TYPE_VIDEO_CAPTURE};
  bool streaming_{false};
  std::vector<Buffer> bufs_;
};

static bool save_file(const std::string &path, const std::vector<uint8_t> &data) {
  FILE *fp = std::fopen(path.c_str(), "wb");
  if (!fp) return false;
  bool ok = std::fwrite(data.data(), 1, data.size(), fp) == data.size();
  std::fclose(fp);
  return ok;
}

static bool save_bmp24(const std::string &path, const std::vector<uint8_t> &rgb, int w, int h) {
  if (rgb.empty()) return false;
  int row_stride = ((w * 3 + 3) / 4) * 4;
  int data_size = row_stride * h;
  int file_size = 54 + data_size;
  std::vector<uint8_t> out(file_size);
  out[0] = 'B';
  out[1] = 'M';
  *reinterpret_cast<uint32_t *>(&out[2]) = file_size;
  *reinterpret_cast<uint32_t *>(&out[10]) = 54;
  *reinterpret_cast<uint32_t *>(&out[14]) = 40;
  *reinterpret_cast<int32_t *>(&out[18]) = w;
  *reinterpret_cast<int32_t *>(&out[22]) = h;
  *reinterpret_cast<uint16_t *>(&out[26]) = 1;
  *reinterpret_cast<uint16_t *>(&out[28]) = 24;
  *reinterpret_cast<uint32_t *>(&out[34]) = data_size;
  for (int y = 0; y < h; ++y) {
    uint8_t *dst = out.data() + 54 + (h - 1 - y) * row_stride;
    const uint8_t *src = rgb.data() + static_cast<size_t>(y) * w * 3;
    for (int x = 0; x < w; ++x) {
      dst[x * 3 + 0] = src[x * 3 + 2];
      dst[x * 3 + 1] = src[x * 3 + 1];
      dst[x * 3 + 2] = src[x * 3 + 0];
    }
  }
  return save_file(path, out);
}

static bool load_bmp24_rgb(const std::string &path, std::vector<uint8_t> &rgb, int &w, int &h) {
  FILE *fp = std::fopen(path.c_str(), "rb");
  if (!fp) return false;
  uint8_t header[54]{};
  if (std::fread(header, 1, sizeof(header), fp) != sizeof(header) ||
      header[0] != 'B' || header[1] != 'M') {
    std::fclose(fp);
    return false;
  }
  uint32_t data_offset = *reinterpret_cast<uint32_t *>(&header[10]);
  int32_t bmp_w = *reinterpret_cast<int32_t *>(&header[18]);
  int32_t bmp_h_raw = *reinterpret_cast<int32_t *>(&header[22]);
  uint16_t bits = *reinterpret_cast<uint16_t *>(&header[28]);
  if (bmp_w <= 0 || bmp_h_raw == 0 || (bits != 24 && bits != 32)) {
    std::fclose(fp);
    return false;
  }
  w = bmp_w;
  h = std::abs(bmp_h_raw);
  int row_stride = ((w * bits + 31) / 32) * 4;
  rgb.assign(static_cast<size_t>(w) * h * 3, 0);
  std::vector<uint8_t> row(row_stride);
  std::fseek(fp, data_offset, SEEK_SET);
  for (int row_i = 0; row_i < h; ++row_i) {
    if (std::fread(row.data(), 1, row.size(), fp) != row.size()) break;
    int y = bmp_h_raw > 0 ? (h - 1 - row_i) : row_i;
    for (int x = 0; x < w; ++x) {
      const uint8_t *src = row.data() + x * (bits / 8);
      uint8_t *dst = rgb.data() + (static_cast<size_t>(y) * w + x) * 3;
      dst[0] = src[2];
      dst[1] = src[1];
      dst[2] = src[0];
    }
  }
  std::fclose(fp);
  return true;
}

static bool load_image_rgb(const std::string &path, std::vector<uint8_t> &rgb, int &w, int &h) {
  int comp = 0;
  stbi_uc *data = stbi_load(path.c_str(), &w, &h, &comp, 3);
  if (!data) return false;
  rgb.assign(data, data + static_cast<size_t>(w) * h * 3);
  stbi_image_free(data);
  return true;
}

static bool save_png24(const std::string &path, const std::vector<uint8_t> &rgb, int w, int h) {
  if (rgb.empty() || w <= 0 || h <= 0) return false;
  return stbi_write_png(path.c_str(), w, h, 3, rgb.data(), w * 3) != 0;
}

struct Detection {
  int left{};
  int top{};
  int right{};
  int bottom{};
  int cls{};
  float score{};
  std::string label;
};

static float sigmoid(float x) { return 1.0f / (1.0f + std::exp(-x)); }

static float deqnt_i8(int8_t q, int zp, float scale) {
  return (static_cast<float>(q) - static_cast<float>(zp)) * scale;
}

static float overlap_iou(const Detection &a, const Detection &b) {
  int xx1 = std::max(a.left, b.left);
  int yy1 = std::max(a.top, b.top);
  int xx2 = std::min(a.right, b.right);
  int yy2 = std::min(a.bottom, b.bottom);
  int w = std::max(0, xx2 - xx1 + 1);
  int h = std::max(0, yy2 - yy1 + 1);
  float inter = static_cast<float>(w * h);
  float area_a = static_cast<float>((a.right - a.left + 1) * (a.bottom - a.top + 1));
  float area_b = static_cast<float>((b.right - b.left + 1) * (b.bottom - b.top + 1));
  float denom = area_a + area_b - inter;
  return denom <= 0.0f ? 0.0f : inter / denom;
}

class YoloDetector {
public:
  bool init(const std::string &model_path, const std::string &label_path) {
    load_labels(label_path);
    auto model = read_all(model_path);
    if (model.empty()) {
      std::fprintf(stderr, "YOLO model missing: %s\n", model_path.c_str());
      return false;
    }
    int ret = rknn_init(&ctx_, model.data(), static_cast<uint32_t>(model.size()), 0, nullptr);
    if (ret != 0) {
      std::fprintf(stderr, "rknn_init YOLO failed: %d\n", ret);
      return false;
    }
    ret = rknn_query(ctx_, RKNN_QUERY_IN_OUT_NUM, &io_num_, sizeof(io_num_));
    if (ret != 0 || io_num_.n_input < 1 || io_num_.n_output < 3) {
      std::fprintf(stderr, "rknn_query io failed: %d in=%u out=%u\n", ret, io_num_.n_input, io_num_.n_output);
      release();
      return false;
    }
    input_attrs_.resize(io_num_.n_input);
    output_attrs_.resize(io_num_.n_output);
    for (uint32_t i = 0; i < io_num_.n_input; ++i) {
      input_attrs_[i].index = i;
      rknn_query(ctx_, RKNN_QUERY_INPUT_ATTR, &input_attrs_[i], sizeof(rknn_tensor_attr));
    }
    for (uint32_t i = 0; i < io_num_.n_output; ++i) {
      output_attrs_[i].index = i;
      rknn_query(ctx_, RKNN_QUERY_OUTPUT_ATTR, &output_attrs_[i], sizeof(rknn_tensor_attr));
    }
    if (input_attrs_[0].fmt == RKNN_TENSOR_NCHW) {
      model_h_ = static_cast<int>(input_attrs_[0].dims[2]);
      model_w_ = static_cast<int>(input_attrs_[0].dims[3]);
      model_c_ = static_cast<int>(input_attrs_[0].dims[1]);
    } else {
      model_h_ = static_cast<int>(input_attrs_[0].dims[1]);
      model_w_ = static_cast<int>(input_attrs_[0].dims[2]);
      model_c_ = static_cast<int>(input_attrs_[0].dims[3]);
    }
    ready_ = model_w_ > 0 && model_h_ > 0 && model_c_ == 3;
    std::printf("YOLO ready: %dx%d outputs=%u\n", model_w_, model_h_, io_num_.n_output);
    return ready_;
  }

  void release() {
    if (ctx_) {
      rknn_destroy(ctx_);
      ctx_ = 0;
    }
    ready_ = false;
  }

  ~YoloDetector() { release(); }
  bool ready() const { return ready_; }

  std::vector<Detection> infer(const std::vector<uint8_t> &rgb, int src_w, int src_h) {
    std::vector<Detection> empty;
    if (!ready_ || rgb.empty()) return empty;

    float scale = std::min(static_cast<float>(model_w_) / src_w, static_cast<float>(model_h_) / src_h);
    int resized_w = std::max(1, static_cast<int>(src_w * scale));
    int resized_h = std::max(1, static_cast<int>(src_h * scale));
    int x_pad = (model_w_ - resized_w) / 2;
    int y_pad = (model_h_ - resized_h) / 2;
    std::vector<uint8_t> input(static_cast<size_t>(model_w_) * model_h_ * 3, 114);
    for (int y = 0; y < resized_h; ++y) {
      int sy = y * src_h / resized_h;
      for (int x = 0; x < resized_w; ++x) {
        int sx = x * src_w / resized_w;
        const uint8_t *s = &rgb[(static_cast<size_t>(sy) * src_w + sx) * 3];
        uint8_t *d = &input[(static_cast<size_t>(y + y_pad) * model_w_ + (x + x_pad)) * 3];
        d[0] = s[0];
        d[1] = s[1];
        d[2] = s[2];
      }
    }

    rknn_input in{};
    in.index = 0;
    in.type = RKNN_TENSOR_UINT8;
    in.fmt = RKNN_TENSOR_NHWC;
    in.size = static_cast<uint32_t>(input.size());
    in.buf = input.data();
    int ret = rknn_inputs_set(ctx_, 1, &in);
    if (ret != 0) return empty;
    ret = rknn_run(ctx_, nullptr);
    if (ret != 0) return empty;

    std::vector<rknn_output> outputs(io_num_.n_output);
    for (uint32_t i = 0; i < io_num_.n_output; ++i) {
      outputs[i].index = i;
      outputs[i].want_float = 0;
    }
    ret = rknn_outputs_get(ctx_, io_num_.n_output, outputs.data(), nullptr);
    if (ret != 0) return empty;

    std::vector<Detection> dets;
    for (int i = 0; i < 3 && i < static_cast<int>(io_num_.n_output); ++i) {
      int grid_h = static_cast<int>(output_attrs_[i].dims[2]);
      int grid_w = static_cast<int>(output_attrs_[i].dims[3]);
      int stride = model_h_ / grid_h;
      process_output(reinterpret_cast<int8_t *>(outputs[i].buf), anchors_[i], grid_h, grid_w, stride,
                     output_attrs_[i].zp, output_attrs_[i].scale, x_pad, y_pad, scale, src_w, src_h, dets);
    }
    rknn_outputs_release(ctx_, io_num_.n_output, outputs.data());

    std::sort(dets.begin(), dets.end(), [](const Detection &a, const Detection &b) {
      return a.score > b.score;
    });
    std::vector<Detection> kept;
    for (const auto &d : dets) {
      bool drop = false;
      for (const auto &k : kept) {
        if (d.cls == k.cls && overlap_iou(d, k) > YOLO_NMS_THRESH) {
          drop = true;
          break;
        }
      }
      if (!drop) {
        kept.push_back(d);
        if (static_cast<int>(kept.size()) >= YOLO_MAX_BOXES) break;
      }
    }
    return kept;
  }

private:
  static std::vector<uint8_t> read_all(const std::string &path) {
    FILE *fp = std::fopen(path.c_str(), "rb");
    if (!fp) return {};
    std::fseek(fp, 0, SEEK_END);
    long n = std::ftell(fp);
    std::fseek(fp, 0, SEEK_SET);
    std::vector<uint8_t> data(n);
    if (n > 0) std::fread(data.data(), 1, data.size(), fp);
    std::fclose(fp);
    return data;
  }

  void load_labels(const std::string &path) {
    labels_.clear();
    FILE *fp = std::fopen(path.c_str(), "r");
    char line[128];
    while (fp && std::fgets(line, sizeof(line), fp)) {
      std::string s(line);
      while (!s.empty() && (s.back() == '\n' || s.back() == '\r')) s.pop_back();
      if (!s.empty()) labels_.push_back(s);
    }
    if (fp) std::fclose(fp);
    static const char *fallback[] = {"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck"};
    const size_t fallback_count = sizeof(fallback) / sizeof(fallback[0]);
    while (labels_.size() < YOLO_CLASS_NUM) {
      labels_.push_back(labels_.size() < fallback_count ? fallback[labels_.size()] : ("class" + std::to_string(labels_.size())));
    }
  }

  void process_output(int8_t *input, const int *anchor, int grid_h, int grid_w, int stride,
                      int zp, float qscale, int x_pad, int y_pad, float scale,
                      int src_w, int src_h, std::vector<Detection> &dets) {
    int grid_len = grid_h * grid_w;
    for (int a = 0; a < 3; ++a) {
      for (int i = 0; i < grid_h; ++i) {
        for (int j = 0; j < grid_w; ++j) {
          int offset = (YOLO_PROP_BOX_SIZE * a) * grid_len + i * grid_w + j;
          int8_t *p = input + offset;
          float obj = deqnt_i8(p[4 * grid_len], zp, qscale);
          if (obj < YOLO_BOX_THRESH) continue;
          int cls = 0;
          float cls_prob = deqnt_i8(p[5 * grid_len], zp, qscale);
          for (int k = 1; k < YOLO_CLASS_NUM; ++k) {
            float prob = deqnt_i8(p[(5 + k) * grid_len], zp, qscale);
            if (prob > cls_prob) {
              cls_prob = prob;
              cls = k;
            }
          }
          float score = obj * cls_prob;
          if (score < YOLO_BOX_THRESH) continue;
          float bx = deqnt_i8(p[0], zp, qscale) * 2.0f - 0.5f;
          float by = deqnt_i8(p[1 * grid_len], zp, qscale) * 2.0f - 0.5f;
          float bw = deqnt_i8(p[2 * grid_len], zp, qscale) * 2.0f;
          float bh = deqnt_i8(p[3 * grid_len], zp, qscale) * 2.0f;
          bx = (bx + j) * stride;
          by = (by + i) * stride;
          bw = bw * bw * anchor[a * 2];
          bh = bh * bh * anchor[a * 2 + 1];
          float x1 = (bx - bw / 2.0f - x_pad) / scale;
          float y1 = (by - bh / 2.0f - y_pad) / scale;
          float x2 = (bx + bw / 2.0f - x_pad) / scale;
          float y2 = (by + bh / 2.0f - y_pad) / scale;
          Detection d;
          d.left = clamp_int(static_cast<int>(x1), 0, src_w - 1);
          d.top = clamp_int(static_cast<int>(y1), 0, src_h - 1);
          d.right = clamp_int(static_cast<int>(x2), 0, src_w - 1);
          d.bottom = clamp_int(static_cast<int>(y2), 0, src_h - 1);
          d.cls = cls;
          d.score = score;
          d.label = cls < static_cast<int>(labels_.size()) ? labels_[cls] : ("class" + std::to_string(cls));
          if (d.right > d.left && d.bottom > d.top) dets.push_back(d);
        }
      }
    }
  }

  rknn_context ctx_{0};
  bool ready_{false};
  rknn_input_output_num io_num_{};
  std::vector<rknn_tensor_attr> input_attrs_;
  std::vector<rknn_tensor_attr> output_attrs_;
  int model_w_{0};
  int model_h_{0};
  int model_c_{0};
  std::vector<std::string> labels_;
  const int anchors_[3][6] = {
      {10, 13, 16, 30, 33, 23},
      {30, 61, 62, 45, 59, 119},
      {116, 90, 156, 198, 373, 326},
  };
};

static std::string classify_rgb(const std::vector<uint8_t> &rgb) {
  if (rgb.empty()) return "MJPEG SAVED";
  uint64_t r = 0, g = 0, b = 0;
  size_t count = rgb.size() / 3;
  for (size_t i = 0; i < rgb.size(); i += 3) {
    r += rgb[i];
    g += rgb[i + 1];
    b += rgb[i + 2];
  }
  r /= count;
  g /= count;
  b /= count;
  uint64_t brightness = (r + g + b) / 3;
  if (brightness < 50) return "DARK OBJECT";
  if (brightness > 205) return "BRIGHT OBJECT";
  if (r > g + 25 && r > b + 25) return "RED CLASS";
  if (g > r + 25 && g > b + 25) return "GREEN CLASS";
  if (b > r + 25 && b > g + 25) return "BLUE CLASS";
  return "MIXED CLASS";
}

static std::string classify_with_rknn(const std::string &image_path,
                                      const std::vector<uint8_t> &fallback_rgb) {
  const char *model = "/usr/share/model/RK356X/mobilenet_v1.rknn";
  if (!fs::exists(model) || !fs::exists("/usr/bin/rknn_common_test")) {
    return classify_rgb(fallback_rgb);
  }

  std::string cmd = "rknn_common_test " + std::string(model) + " " + image_path + " 1 2>/dev/null";
  FILE *fp = popen(cmd.c_str(), "r");
  if (!fp) return classify_rgb(fallback_rgb);

  char line[256];
  bool top = false;
  std::string result;
  while (std::fgets(line, sizeof(line), fp)) {
    std::string s(line);
    if (s.find("---- Top5 ----") != std::string::npos) {
      top = true;
      continue;
    }
    if (top) {
      std::istringstream iss(s);
      double prob = 0.0;
      std::string dash;
      int cls = -1;
      if (iss >> prob >> dash >> cls) {
        char buf[64];
        std::snprintf(buf, sizeof(buf), "RKNN C%d %.2f", cls, prob);
        result = buf;
        break;
      }
    }
  }
  int rc = pclose(fp);
  if (rc != 0 || result.empty()) return classify_rgb(fallback_rgb);
  return result;
}

static void draw_button(FbDisplay &fb, const Button &b) {
  fb.fill_rect(b.x, b.y, b.w, b.h, blend_rgb(b.color, C_LEATHER, 0.18f));
  fb.draw_rect(b.x, b.y, b.w, b.h, C_GOLD);
  fb.draw_rect(b.x + 1, b.y + 1, b.w - 2, b.h - 2, C_LEATHER_2);
  int scale = std::max(2, b.h / 35);
  int tx = b.x + std::max(8, (b.w - static_cast<int>(b.label.size()) * 6 * scale) / 2);
  int ty = b.y + (b.h - 7 * scale) / 2;
  fb.draw_text(tx, ty, b.label, C_TEXT, scale);
}

static Button wifi_top_button(int screen_w) {
  return {"WIFI", screen_w - 128, 18, 108, 48, C_LEATHER};
}

static Button gallery_top_button(int screen_w) {
  return {"GALLERY", screen_w - 260, 18, 120, 48, C_LEATHER};
}

static void draw_ui(FbDisplay &fb, const std::string &status, const std::string &result,
                    const std::vector<Button> &buttons) {
  fb.clear(C_BG);
  fb.draw_text(20, 18, "RK356X CCD CAMERA", C_TEXT, 3);
  fb.draw_text(22, 56, "LCD TOUCH CAMERA AI", C_MUTED, 2);
  draw_button(fb, gallery_top_button(fb.width()));
  draw_button(fb, wifi_top_button(fb.width()));
  int preview_h = std::max(120, fb.height() - 170);
  fill_leather_panel(fb, 20, 90, fb.width() - 40, preview_h, 0x3a281f, C_LEATHER);
  fb.draw_text(38, 110, "PREVIEW / SNAPSHOT", C_MUTED, 2);
  fb.draw_text(38, 145, "STATUS: " + status, C_TEXT, 2);
  fb.draw_text(38, 178, "RESULT: " + result, C_GOLD, 2);
  for (const auto &b : buttons) draw_button(fb, b);
}

static void draw_main_camera_ui(FbDisplay &fb, const std::string &status, const std::string &result,
                                const std::vector<Button> &buttons, bool recording,
                                const std::string &record_label) {
  fb.clear(C_BG);
  fb.draw_text(24, 18, "RK356X CCD CAMERA", C_TEXT, 3);
  fb.draw_text(24, 54, "LCD TOUCH CAMERA AI", C_MUTED, 2);
  draw_button(fb, gallery_top_button(fb.width()));
  draw_button(fb, wifi_top_button(fb.width()));

  int left_x = 20;
  int left_y = 96;
  int left_w = std::max(220, fb.width() / 4);
  int left_h = fb.height() - 118;
  int right_x = left_x + left_w + 18;
  int right_y = 96;
  int right_w = fb.width() - right_x - 20;
  int right_h = fb.height() - 118;

  fill_leather_panel(fb, left_x, left_y, left_w, left_h, 0x3a281f, C_LEATHER);
  fb.draw_text(left_x + 18, left_y + 18, "CONTROL", C_ACCENT, 2);
  fb.draw_text(left_x + 18, left_y + 48, "STATUS: " + clip_text(status, 28), C_TEXT, 2);
  fb.draw_text(left_x + 18, left_y + 78, "RESULT: " + clip_text(result, 28), C_MUTED, 2);
  fb.draw_text(left_x + 18, left_y + 110, recording ? "RECORDING" : "IDLE", recording ? C_WARN : C_MUTED, 2);
  fb.draw_text(left_x + 18, left_y + 140, "REC FILES", C_TEXT, 2);
  fb.draw_text(left_x + 18, left_y + 170, clip_text(record_label, 30), C_ACCENT, 2);

  int by = left_y + 210;
  for (const auto &b : buttons) {
    Button shifted = b;
    shifted.x = left_x + 18;
    shifted.w = left_w - 36;
    shifted.y = by;
    shifted.h = buttons.size() > 4 ? 42 : 48;
    draw_button(fb, shifted);
    by += buttons.size() > 4 ? 48 : 58;
  }

  fill_leather_panel(fb, right_x, right_y, right_w, right_h, 0x3a281f, C_LEATHER);
  fb.draw_text(right_x + 18, right_y + 18, "PREVIEW", C_MUTED, 2);
  fb.flush();
}

static void draw_detections(FbDisplay &fb, const std::vector<Detection> &dets,
                            int px, int py, int pw, int ph, int src_w, int src_h) {
  for (const auto &d : dets) {
    int x1 = px + d.left * pw / src_w;
    int y1 = py + d.top * ph / src_h;
    int x2 = px + d.right * pw / src_w;
    int y2 = py + d.bottom * ph / src_h;
    fb.draw_rect(x1, y1, std::max(4, x2 - x1), std::max(4, y2 - y1), C_ACCENT);
    fb.draw_rect(x1 + 2, y1 + 2, std::max(4, x2 - x1 - 4), std::max(4, y2 - y1 - 4), C_ACCENT);
    char text[96];
    std::snprintf(text, sizeof(text), "%s %.0f%%", d.label.c_str(), d.score * 100.0f);
    int label_w = std::min(static_cast<int>(std::strlen(text)) * 12 + 8, pw);
    int label_y = std::max(py, y1 - 22);
    fb.fill_rect(x1, label_y, label_w, 20, 0x36261d);
    fb.draw_text(x1 + 4, label_y + 4, text, C_TEXT, 2);
  }
}

static void put_rgb_pixel(std::vector<uint8_t> &rgb, int w, int h, int x, int y,
                          uint8_t r, uint8_t g, uint8_t b) {
  if (x < 0 || y < 0 || x >= w || y >= h) return;
  uint8_t *p = rgb.data() + (static_cast<size_t>(y) * w + x) * 3;
  p[0] = r;
  p[1] = g;
  p[2] = b;
}

static void draw_rgb_rect(std::vector<uint8_t> &rgb, int w, int h, int x1, int y1, int x2, int y2,
                          uint8_t r = 49, uint8_t g = 196, uint8_t b = 141) {
  x1 = clamp_int(x1, 0, w - 1);
  y1 = clamp_int(y1, 0, h - 1);
  x2 = clamp_int(x2, 0, w - 1);
  y2 = clamp_int(y2, 0, h - 1);
  if (x2 <= x1 || y2 <= y1) return;
  for (int t = 0; t < 4; ++t) {
    for (int x = x1; x <= x2; ++x) {
      put_rgb_pixel(rgb, w, h, x, y1 + t, r, g, b);
      put_rgb_pixel(rgb, w, h, x, y2 - t, r, g, b);
    }
    for (int y = y1; y <= y2; ++y) {
      put_rgb_pixel(rgb, w, h, x1 + t, y, r, g, b);
      put_rgb_pixel(rgb, w, h, x2 - t, y, r, g, b);
    }
  }
}

static std::vector<uint8_t> annotate_rgb(const std::vector<uint8_t> &rgb, int w, int h,
                                         const std::vector<Detection> &dets) {
  std::vector<uint8_t> out = rgb;
  for (const auto &d : dets) {
    draw_rgb_rect(out, w, h, d.left, d.top, d.right, d.bottom);
  }
  return out;
}

static int64_t monotonic_ms() {
  timespec ts{};
  clock_gettime(CLOCK_MONOTONIC, &ts);
  return static_cast<int64_t>(ts.tv_sec) * 1000 + ts.tv_nsec / 1000000;
}

static std::string timestamp_name(const std::string &prefix) {
  time_t now = std::time(nullptr);
  tm tm_now{};
  localtime_r(&now, &tm_now);
  char buf[96];
  static int seq = 0;
  std::snprintf(buf, sizeof(buf), "%s_%04d%02d%02d_%02d%02d%02d_%02d.bmp",
                prefix.c_str(), tm_now.tm_year + 1900, tm_now.tm_mon + 1, tm_now.tm_mday,
                tm_now.tm_hour, tm_now.tm_min, tm_now.tm_sec, seq++ % 100);
  return buf;
}

static std::string timestamp_slug(const std::string &prefix) {
  std::string name = timestamp_name(prefix);
  if (name.size() > 4 && name.substr(name.size() - 4) == ".bmp") {
    name.resize(name.size() - 4);
  }
  return name;
}

static std::string save_gallery_capture(const std::vector<uint8_t> &rgb, int w, int h,
                                        const std::vector<Detection> &dets,
                                        const std::string &prefix) {
  const std::string dir = "/root/rk356x_demo/gallery";
  fs::create_directories(dir);
  std::vector<uint8_t> annotated = annotate_rgb(rgb, w, h, dets);
  std::string path = dir + "/" + timestamp_name(prefix);
  if (!save_bmp24(path, annotated, w, h)) return {};
  save_bmp24("/tmp/rk_capture.bmp", annotated, w, h);
  return path;
}

struct GalleryItem {
  std::string path;
  std::string name;
};

struct VideoSession {
  std::string dir;
  std::string name;
  int frame_count{0};
};

struct RecordSession {
  std::string dir;
  int frame_count{0};
  bool active{false};
  int64_t last_write_ms{0};

  bool start() {
    stop();
    const std::string base = "/root/rk356x_demo/recordings";
    fs::create_directories(base);
    dir = base + "/" + timestamp_slug("session");
    if (!fs::create_directories(dir)) return false;
    frame_count = 0;
    active = true;
    last_write_ms = 0;
    FILE *meta = std::fopen((dir + "/meta.txt").c_str(), "w");
    if (meta) {
      std::fprintf(meta, "format=png\nfps=5\n");
      std::fclose(meta);
    }
    FILE *latest = std::fopen((base + "/latest.txt").c_str(), "w");
    if (latest) {
      std::fprintf(latest, "%s\n", dir.c_str());
      std::fclose(latest);
    }
    return true;
  }

  void stop() { active = false; }

  bool record(const std::vector<uint8_t> &rgb, int w, int h,
              const std::vector<Detection> &dets, int64_t now_ms) {
    if (!active || rgb.empty() || w <= 0 || h <= 0) return false;
    if (now_ms - last_write_ms < 180) return false;
    std::vector<uint8_t> annotated = annotate_rgb(rgb, w, h, dets);
    char name[128];
    std::snprintf(name, sizeof(name), "%s/frame_%06d.png", dir.c_str(), frame_count++);
    last_write_ms = now_ms;
    return save_png24(name, annotated, w, h);
  }
};

static std::string record_session_title(const std::string &dir) {
  if (dir.empty()) return "NO SESSION";
  return clip_text(fs::path(dir).filename().string(), 40);
}

static bool is_image_file(const fs::path &path) {
  std::string ext = path.extension().string();
  std::transform(ext.begin(), ext.end(), ext.begin(), [](unsigned char c) { return std::tolower(c); });
  return ext == ".bmp" || ext == ".png" || ext == ".jpg" || ext == ".jpeg";
}

static std::vector<GalleryItem> load_gallery_items() {
  std::vector<GalleryItem> items;
  const std::string dir = "/root/rk356x_demo/gallery";
  fs::create_directories(dir);
  for (const auto &entry : fs::directory_iterator(dir)) {
    if (!entry.is_regular_file() || !is_image_file(entry.path())) continue;
    items.push_back({entry.path().string(), entry.path().filename().string()});
  }
  std::sort(items.begin(), items.end(), [](const GalleryItem &a, const GalleryItem &b) {
    return a.name > b.name;
  });
  return items;
}

static std::vector<GalleryItem> load_video_frames(const std::string &dir) {
  std::vector<GalleryItem> items;
  if (dir.empty() || !fs::exists(dir)) return items;
  for (const auto &entry : fs::directory_iterator(dir)) {
    if (!entry.is_regular_file() || !is_image_file(entry.path())) continue;
    items.push_back({entry.path().string(), entry.path().filename().string()});
  }
  std::sort(items.begin(), items.end(), [](const GalleryItem &a, const GalleryItem &b) {
    return a.name < b.name;
  });
  return items;
}

static std::vector<VideoSession> load_video_sessions() {
  std::vector<VideoSession> sessions;
  const std::string base = "/root/rk356x_demo/recordings";
  fs::create_directories(base);
  for (const auto &entry : fs::directory_iterator(base)) {
    if (!entry.is_directory()) continue;
    auto frames = load_video_frames(entry.path().string());
    if (frames.empty()) continue;
    sessions.push_back({entry.path().string(), entry.path().filename().string(), static_cast<int>(frames.size())});
  }
  std::sort(sessions.begin(), sessions.end(), [](const VideoSession &a, const VideoSession &b) {
    return a.name > b.name;
  });
  return sessions;
}

static void draw_gallery_tabs(FbDisplay &fb, bool video_tab) {
  Button photo{"PHOTO", fb.width() - 286, 18, 126, 44, video_tab ? C_LEATHER : C_ACCENT};
  Button video{"VIDEO", fb.width() - 150, 18, 126, 44, video_tab ? C_ACCENT : C_LEATHER};
  draw_button(fb, photo);
  draw_button(fb, video);
}

static void draw_gallery(FbDisplay &fb, const std::vector<GalleryItem> &items, int index,
                         const std::string &message, const std::vector<Button> &buttons,
                         const std::string &title = "GALLERY") {
  fb.clear(C_BG);
  fb.draw_text(20, 18, title, C_TEXT, 4);
  draw_gallery_tabs(fb, false);
  char count[64];
  std::snprintf(count, sizeof(count), "%d / %zu", items.empty() ? 0 : index + 1, items.size());
  fb.draw_text(22, 62, count, C_ACCENT, 2);
  fb.draw_text(160, 62, clip_text(message, 52), C_MUTED, 2);

  int area_x = 20;
  int area_y = 96;
  int area_w = fb.width() - 40;
  int area_h = fb.height() - 190;
  fill_leather_panel(fb, area_x, area_y, area_w, area_h, 0x3a281f, C_LEATHER);
  if (items.empty()) {
    fb.draw_text(area_x + 24, area_y + 34, "NO CAPTURED IMAGE", C_MUTED, 3);
  } else {
    std::vector<uint8_t> img;
    int iw = 0, ih = 0;
    if (load_image_rgb(items[index].path, img, iw, ih)) {
      float scale = std::min(static_cast<float>(area_w - 24) / iw, static_cast<float>(area_h - 54) / ih);
      int dw = std::max(1, static_cast<int>(iw * scale));
      int dh = std::max(1, static_cast<int>(ih * scale));
      int dx = area_x + (area_w - dw) / 2;
      int dy = area_y + 12;
      fb.draw_rgb(dx, dy, dw, dh, img, iw, ih);
      fb.draw_text(area_x + 18, area_y + area_h - 30, clip_text(items[index].name, 70), C_TEXT, 2);
    } else {
      fb.draw_text(area_x + 24, area_y + 34, "LOAD IMAGE FAILED", C_DANGER, 3);
    }
  }
  for (const auto &b : buttons) draw_button(fb, b);
  fb.flush();
}

static void draw_video_gallery(FbDisplay &fb, const std::vector<VideoSession> &sessions, int video_index,
                               const std::vector<GalleryItem> &frames, int frame_index,
                               bool playing, const std::string &message,
                               const std::vector<Button> &buttons) {
  fb.clear(C_BG);
  fb.draw_text(20, 18, "ALBUM", C_TEXT, 4);
  draw_gallery_tabs(fb, true);
  char count[96];
  std::snprintf(count, sizeof(count), "%d / %zu", sessions.empty() ? 0 : video_index + 1, sessions.size());
  fb.draw_text(22, 62, count, C_ACCENT, 2);
  fb.draw_text(160, 62, clip_text(message, 52), C_MUTED, 2);

  int area_x = 20;
  int area_y = 96;
  int area_w = fb.width() - 40;
  int area_h = fb.height() - 190;
  fb.fill_rect(area_x, area_y, area_w, area_h, 0x36261d);
  fill_leather_panel(fb, area_x, area_y, area_w, area_h, 0x3a281f, C_LEATHER);

  if (sessions.empty() || frames.empty()) {
    fb.draw_text(area_x + 24, area_y + 34, "NO RECORDED VIDEO", C_MUTED, 3);
  } else {
    std::vector<uint8_t> img;
    int iw = 0, ih = 0;
    int safe_frame = clamp_int(frame_index, 0, static_cast<int>(frames.size()) - 1);
    if (load_image_rgb(frames[safe_frame].path, img, iw, ih)) {
      float scale = std::min(static_cast<float>(area_w - 24) / iw, static_cast<float>(area_h - 82) / ih);
      int dw = std::max(1, static_cast<int>(iw * scale));
      int dh = std::max(1, static_cast<int>(ih * scale));
      int dx = area_x + (area_w - dw) / 2;
      int dy = area_y + 12;
      fb.draw_rgb(dx, dy, dw, dh, img, iw, ih);
      if (!playing) {
        int pcx = area_x + area_w / 2;
        int pcy = area_y + area_h / 2 - 12;
        fb.fill_rect(pcx - 38, pcy - 38, 76, 76, blend_rgb(C_BG, C_LEATHER, 0.30f));
        fb.draw_rect(pcx - 38, pcy - 38, 76, 76, C_ACCENT);
        fb.draw_play_triangle(pcx + 8, pcy, 30, C_TEXT);
      }
    } else {
      fb.draw_text(area_x + 24, area_y + 34, "LOAD VIDEO FRAME FAILED", C_DANGER, 3);
    }
    int bar_x = area_x + 18;
    int bar_y = area_y + area_h - 54;
    int bar_w = area_w - 36;
    int fill_w = frames.empty() ? 0 : (bar_w * (safe_frame + 1) / static_cast<int>(frames.size()));
    fb.fill_rect(bar_x, bar_y, bar_w, 12, blend_rgb(C_BG, C_LEATHER, 0.45f));
    fb.fill_rect(bar_x, bar_y, fill_w, 12, C_ACCENT);
    fb.draw_rect(bar_x, bar_y, bar_w, 12, C_TEXT);
    char info[160];
    std::snprintf(info, sizeof(info), "%s  FRAME %d/%zu  %s", sessions[video_index].name.c_str(),
                  safe_frame + 1, frames.size(), playing ? "PLAYING" : "PAUSED");
    fb.draw_text(area_x + 18, area_y + area_h - 30, clip_text(info, 78), C_TEXT, 2);
  }

  for (const auto &b : buttons) draw_button(fb, b);
  fb.flush();
}

struct WifiNetwork {
  std::string ssid;
  int signal{-1000};
};

static std::string shell_capture(const std::string &cmd) {
  FILE *fp = popen(cmd.c_str(), "r");
  if (!fp) return {};
  std::string out;
  char buf[256];
  while (std::fgets(buf, sizeof(buf), fp)) out += buf;
  pclose(fp);
  return trim_copy(out);
}

static std::string wifi_conf_escape(const std::string &s) {
  std::string out;
  for (char c : s) {
    if (c == '\\' || c == '"') out.push_back('\\');
    out.push_back(c);
  }
  return out;
}

static std::string current_ip() {
  std::string ip = shell_capture("ip -4 addr show wlan0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -1");
  return ip.empty() ? "NO IP" : ip;
}

static std::vector<WifiNetwork> scan_wifi() {
  std::vector<WifiNetwork> nets;
  FILE *fp = popen("ip link set wlan0 up 2>/dev/null; iw dev wlan0 scan 2>/dev/null", "r");
  if (!fp) return nets;
  char line[512];
  int current_signal = -1000;
  while (std::fgets(line, sizeof(line), fp)) {
    std::string s(line);
    size_t sig_pos = s.find("signal:");
    if (sig_pos != std::string::npos) {
      current_signal = std::atoi(s.c_str() + sig_pos + 7);
      continue;
    }
    size_t ssid_pos = s.find("SSID:");
    if (ssid_pos == std::string::npos) continue;
    std::string ssid = trim_copy(s.substr(ssid_pos + 5));
    if (ssid.empty()) continue;
    bool found = false;
    for (auto &n : nets) {
      if (n.ssid == ssid) {
        n.signal = std::max(n.signal, current_signal);
        found = true;
        break;
      }
    }
    if (!found) nets.push_back({ssid, current_signal});
  }
  pclose(fp);
  std::sort(nets.begin(), nets.end(), [](const WifiNetwork &a, const WifiNetwork &b) {
    return a.signal > b.signal;
  });
  if (nets.size() > 10) nets.resize(10);
  return nets;
}

static bool connect_wifi(const std::string &ssid, const std::string &password) {
  if (ssid.empty() || password.size() < 8) return false;
  FILE *fp = std::fopen("/etc/wpa_supplicant.conf", "w");
  if (!fp) return false;
  std::fprintf(fp,
               "ctrl_interface=/var/run/wpa_supplicant\n"
               "update_config=1\n"
               "ap_scan=1\n\n"
               "network={\n"
               "    ssid=\"%s\"\n"
               "    psk=\"%s\"\n"
               "    key_mgmt=WPA-PSK\n"
               "}\n",
               wifi_conf_escape(ssid).c_str(), wifi_conf_escape(password).c_str());
  std::fclose(fp);
  int rc = std::system(
      "killall wpa_supplicant 2>/dev/null || true; "
      "ip link set wlan0 up 2>/dev/null; "
      "wpa_supplicant -B -Dnl80211 -iwlan0 -c/etc/wpa_supplicant.conf >/tmp/wifi_connect.log 2>&1; "
      "sleep 6; "
      "udhcpc -i wlan0 -n -q -t 6 >>/tmp/wifi_connect.log 2>&1 || dhcpcd -n wlan0 >>/tmp/wifi_connect.log 2>&1");
  return rc == 0 && current_ip() != "NO IP";
}

static void draw_wifi_list(FbDisplay &fb, const std::vector<WifiNetwork> &nets,
                           const std::string &message, const std::vector<Button> &buttons) {
  fb.clear(C_BG);
  fb.draw_text(20, 18, "WIFI", C_TEXT, 4);
  fb.draw_text(22, 62, "IP: " + current_ip(), C_ACCENT, 2);
  fb.draw_text(22, 92, "STATUS: " + clip_text(message, 44), C_TEXT, 2);
  fill_leather_panel(fb, 20, 128, fb.width() - 40, fb.height() - 220, 0x3a281f, C_LEATHER);
  int row_h = 42;
  int y = 142;
  for (size_t i = 0; i < nets.size(); ++i) {
    uint32_t row_color = (i % 2 == 0) ? blend_rgb(C_BG, C_LEATHER, 0.38f)
                                      : blend_rgb(C_BG, C_LEATHER, 0.28f);
    fb.fill_rect(32, y, fb.width() - 64, row_h - 6, row_color);
    char sig[32];
    std::snprintf(sig, sizeof(sig), "%d", nets[i].signal);
    fb.draw_text(44, y + 9, clip_text(nets[i].ssid, 38), C_TEXT, 2);
    fb.draw_text(fb.width() - 150, y + 9, sig, C_MUTED, 2);
    y += row_h;
  }
  if (nets.empty()) fb.draw_text(44, 150, "NO WIFI LIST - TAP SCAN", C_MUTED, 2);
  for (const auto &b : buttons) draw_button(fb, b);
  fb.flush();
}

static void draw_wifi_password(FbDisplay &fb, const std::string &ssid, const std::string &password,
                               bool upper, const std::string &message, const std::vector<Button> &buttons) {
  fb.clear(C_BG);
  fb.draw_text(20, 18, "WIFI PASSWORD", C_TEXT, 3);
  fb.draw_text(22, 58, "SSID: " + clip_text(ssid, 42), C_ACCENT, 2);
  fb.draw_text(22, 88, "PASS: " + std::string(password.size(), '*'), C_TEXT, 2);
  fb.draw_text(22, 118, clip_text(message, 50), C_MUTED, 2);

  const std::string keys = upper ? "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-" :
                                  "abcdefghijklmnopqrstuvwxyz0123456789._-";
  int cols = 10;
  int cell_w = (fb.width() - 64) / cols;
  int cell_h = 42;
  int start_x = 32;
  int start_y = 158;
  for (size_t i = 0; i < keys.size(); ++i) {
    int col = static_cast<int>(i) % cols;
    int row = static_cast<int>(i) / cols;
    int x = start_x + col * cell_w;
    int y = start_y + row * cell_h;
    fb.fill_rect(x + 2, y + 2, cell_w - 4, cell_h - 4, blend_rgb(C_BG, C_LEATHER, 0.35f));
    fb.draw_rect(x + 2, y + 2, cell_w - 4, cell_h - 4, C_LEATHER);
    std::string ch(1, keys[i]);
    fb.draw_text(x + cell_w / 2 - 6, y + 12, ch, C_TEXT, 2);
  }
  for (const auto &b : buttons) draw_button(fb, b);
  fb.flush();
}

int main() {
  std::setvbuf(stdout, nullptr, _IOLBF, 0);
  FbDisplay fb;
  bool has_fb = fb.open_dev();
  if (!has_fb) {
    std::fprintf(stderr, "Running headless: framebuffer unavailable.\n");
  }
  int sw = has_fb ? fb.width() : 800;
  int sh = has_fb ? fb.height() : 480;
  int btn_y = sh - 68;
  int gap = 12;
  int btn_w = (sw - 40 - gap * 3) / 4;
  int pass_btn_w = (sw - 40 - gap * 3) / 4;
  std::vector<Button> buttons{
      {"OPEN", 20, btn_y, btn_w, 52, C_ACCENT},
      {"SNAP", 20 + btn_w + gap, btn_y, btn_w, 52, C_WARN},
      {"CLOSE", 20 + (btn_w + gap) * 2, btn_y, btn_w, 52, C_DANGER},
      {"START REC", 20, btn_y, btn_w, 52, C_DANGER},
  };

  TouchInput touch;
  bool has_touch = touch.open_auto(sw, sh);
  Camera cam;
  YoloDetector yolo;
  bool yolo_ready = yolo.init("/root/rk356x_demo/model/yolov5s_rk3568.rknn",
                             "/root/rk356x_demo/model/coco_80_labels_list.txt");
  std::string status = has_touch ? "READY" : "NO TOUCH";
  std::string result = yolo_ready ? "YOLO READY" : "YOLO OFF";
  std::vector<Detection> detections;
  int preview_x = 32;
  int preview_y = 102;
  int preview_w = sw - 64;
  int preview_h = std::max(80, sh - 220);

  std::printf("Demo started. Touch buttons or press keys: o=open, s=snap, c=close, q=quit.\n");
  enum class View { Main, WifiList, WifiPassword, Gallery };
  View view = View::Main;
  bool running = true;
  bool auto_open = true;
  int frame_counter = 0;
  std::vector<WifiNetwork> wifi_networks;
  std::string wifi_message = "TAP SCAN";
  std::string selected_ssid;
  std::string wifi_password;
  bool wifi_upper = false;
  std::vector<Button> wifi_list_buttons{
      {"BACK", 20, btn_y, btn_w, 52, C_DANGER},
      {"SCAN", 20 + btn_w + gap, btn_y, btn_w, 52, C_ACCENT},
      {"IP", 20 + (btn_w + gap) * 2, btn_y, btn_w, 52, C_LEATHER},
  };
  std::vector<Button> wifi_pass_buttons{
      {"BACK", 20, btn_y, pass_btn_w, 52, C_DANGER},
      {"DEL", 20 + pass_btn_w + gap, btn_y, pass_btn_w, 52, C_WARN},
      {"CASE", 20 + (pass_btn_w + gap) * 2, btn_y, pass_btn_w, 52, C_LEATHER},
      {"OK", 20 + (pass_btn_w + gap) * 3, btn_y, pass_btn_w, 52, C_ACCENT},
  };
  std::vector<GalleryItem> gallery_items = load_gallery_items();
  int gallery_index = 0;
  std::string gallery_message = "TAP SNAP TO ADD";
  RecordSession recorder;
  std::vector<VideoSession> video_sessions = load_video_sessions();
  std::vector<GalleryItem> video_frames;
  int video_index = 0;
  int video_frame_index = 0;
  bool video_playing = false;
  int64_t last_video_frame_ms = 0;
  std::string record_message = "NO RECORD";
  bool gallery_video_tab = false;
  bool record_enabled = false;
  bool main_shell_dirty = true;
  auto draw_main_shell = [&]() {
    if (!has_fb) return;
    draw_main_camera_ui(fb, status, result, buttons, record_enabled, record_message);
    main_shell_dirty = false;
  };
  draw_main_shell();
  std::vector<Button> gallery_buttons{
      {"BACK", 20, btn_y, pass_btn_w, 52, C_DANGER},
      {"PREV", 20 + pass_btn_w + gap, btn_y, pass_btn_w, 52, C_LEATHER},
      {"NEXT", 20 + (pass_btn_w + gap) * 2, btn_y, pass_btn_w, 52, C_LEATHER},
      {"DEL", 20 + (pass_btn_w + gap) * 3, btn_y, pass_btn_w, 52, C_WARN},
  };
  std::vector<Button> video_buttons{
      {"BACK", 20, btn_y, pass_btn_w, 52, C_DANGER},
      {"PREV", 20 + pass_btn_w + gap, btn_y, pass_btn_w, 52, C_LEATHER},
      {"NEXT", 20 + (pass_btn_w + gap) * 2, btn_y, pass_btn_w, 52, C_LEATHER},
      {"PLAY", 20 + (pass_btn_w + gap) * 3, btn_y, pass_btn_w, 52, C_ACCENT},
  };

  std::vector<Button> main_left_buttons{
      {"OPEN", 0, 0, 0, 0, C_ACCENT},
      {"SNAP", 0, 0, 0, 0, C_WARN},
      {"CLOSE", 0, 0, 0, 0, C_DANGER},
      {"START REC", 0, 0, 0, 0, C_DANGER},
  };

  auto update_record_button = [&]() {
    buttons[3].label = record_enabled ? "STOP REC" : "START REC";
    buttons[3].color = record_enabled ? C_WARN : C_DANGER;
    main_left_buttons[3].label = buttons[3].label;
    main_left_buttons[3].color = buttons[3].color;
  };

  auto layout_main_buttons = [&]() {
    int left_x = 20;
    int left_y = 96;
    int left_w = std::max(220, sw / 4);
    int by = left_y + 210;
    for (auto &b : main_left_buttons) {
      b.x = left_x + 18;
      b.y = by;
      b.w = left_w - 36;
      b.h = 48;
      by += 58;
    }
  };
  layout_main_buttons();

  auto draw_frame = [&](const Frame &frame) {
    if (!has_fb || frame.rgb.empty()) return;
    fb.draw_rgb(preview_x, preview_y, preview_w, preview_h, frame.rgb, frame.width, frame.height);
    draw_detections(fb, detections, preview_x, preview_y, preview_w, preview_h, frame.width, frame.height);
    fb.fill_rect(34, 138, std::min(preview_w - 4, 520), 58, 0x36261d);
    fb.draw_rect(34, 138, std::min(preview_w - 4, 520), 58, C_LEATHER);
    fb.draw_text(38, 145, "STATUS: " + status, C_TEXT, 2);
    fb.draw_text(38, 178, "RESULT: " + result, C_GOLD, 2);
  };

  auto summarize_detections = [&]() {
    if (detections.empty()) {
      result = yolo_ready ? "NO OBJECT" : "YOLO OFF";
    } else {
      const auto &d = detections.front();
      char buf[96];
      std::snprintf(buf, sizeof(buf), "%s %.0f%%", d.label.c_str(), d.score * 100.0f);
      result = buf;
    }
  };

  auto refresh_gallery = [&]() {
    gallery_items = load_gallery_items();
    if (gallery_items.empty()) {
      gallery_index = 0;
    } else {
      gallery_index = clamp_int(gallery_index, 0, static_cast<int>(gallery_items.size()) - 1);
    }
  };

  auto refresh_video_sessions = [&]() {
    video_sessions = load_video_sessions();
    if (video_sessions.empty()) {
      video_index = 0;
      video_frames.clear();
      video_frame_index = 0;
    } else {
      video_index = clamp_int(video_index, 0, static_cast<int>(video_sessions.size()) - 1);
      video_frames = load_video_frames(video_sessions[video_index].dir);
      video_frame_index = clamp_int(video_frame_index, 0, std::max(0, static_cast<int>(video_frames.size()) - 1));
    }
  };

  auto update_video_play_button = [&]() {
    video_buttons[3].label = video_playing ? "PAUSE" : "PLAY";
    video_buttons[3].color = video_playing ? C_WARN : C_ACCENT;
  };

  auto draw_current_gallery = [&]() {
    if (!has_fb) return;
    if (gallery_video_tab) {
      update_video_play_button();
      draw_video_gallery(fb, video_sessions, video_index, video_frames, video_frame_index,
                         video_playing, record_message, video_buttons);
    } else {
      draw_gallery(fb, gallery_items, gallery_index, gallery_message, gallery_buttons);
    }
  };

  auto enter_gallery = [&]() {
    view = View::Gallery;
    gallery_video_tab = false;
    video_playing = false;
    refresh_gallery();
    gallery_message = gallery_items.empty() ? "NO IMAGES" : "CAPTURED IMAGES";
    draw_current_gallery();
  };

  auto enter_video_gallery = [&]() {
    view = View::Gallery;
    gallery_video_tab = true;
    video_playing = false;
    refresh_video_sessions();
    record_message = video_sessions.empty() ? "NO VIDEOS" : record_session_title(video_sessions[video_index].dir);
    draw_current_gallery();
  };

  auto do_open = [&]() {
    status = cam.open_auto() ? "CAMERA OPEN" : "OPEN FAILED";
    main_shell_dirty = true;
    draw_main_shell();
  };

  auto do_snap = [&]() {
    if (!cam.is_open()) {
      status = "OPEN CAMERA FIRST";
      main_shell_dirty = true;
      draw_main_shell();
      return;
    }
    Frame frame;
    if (!cam.capture(frame, 1000) || frame.rgb.empty()) {
      status = "CAPTURE FAILED";
      main_shell_dirty = true;
      draw_main_shell();
      return;
    }
    detections = yolo_ready ? yolo.infer(frame.rgb, frame.width, frame.height) : std::vector<Detection>{};
    summarize_detections();
    std::string saved = save_gallery_capture(frame.rgb, frame.width, frame.height, detections, "manual");
    status = "SNAP YOLO";
    if (!saved.empty()) refresh_gallery();
    draw_frame(frame);
    std::printf("%s | %s | boxes=%zu | saved=%s\n", status.c_str(), result.c_str(),
                detections.size(), saved.c_str());
  };

  auto do_close = [&]() {
    cam.close();
    detections.clear();
    status = "CAMERA CLOSED";
    main_shell_dirty = true;
    draw_main_shell();
  };

  auto toggle_record = [&]() {
    if (!record_enabled) {
      record_enabled = recorder.start();
      record_message = record_enabled ? "RECORD STARTED" : "RECORD FAILED";
    } else {
      recorder.stop();
      record_enabled = false;
      refresh_video_sessions();
      record_message = video_sessions.empty() ? "RECORD STOPPED" : ("SAVED " + record_session_title(video_sessions[0].dir));
    }
    update_record_button();
    main_shell_dirty = true;
    draw_main_shell();
  };

  auto do_scan_wifi = [&]() {
    status = "WIFI SCAN";
    wifi_message = "SCANNING...";
    if (has_fb) draw_wifi_list(fb, wifi_networks, wifi_message, wifi_list_buttons);
    wifi_networks = scan_wifi();
    wifi_message = wifi_networks.empty() ? "NO NETWORKS" : ("FOUND " + std::to_string(wifi_networks.size()));
    if (has_fb) draw_wifi_list(fb, wifi_networks, wifi_message, wifi_list_buttons);
  };

  auto enter_wifi = [&]() {
    view = View::WifiList;
    detections.clear();
    wifi_message = "IP " + current_ip();
    if (has_fb) draw_wifi_list(fb, wifi_networks, wifi_message, wifi_list_buttons);
  };

  auto enter_main = [&]() {
    view = View::Main;
    status = "READY";
    if (has_fb) draw_main_camera_ui(fb, status, result, buttons, record_enabled, record_message);
  };

  while (running) {
    if (auto_open) {
      auto_open = false;
      do_open();
    }

    int sx = 0, sy = 0, ex = 0, ey = 0;
    bool swipe = false;
    bool tapped = has_touch && touch.poll_tap(10, sx, sy, swipe, ex, ey);

    char key = 0;
    timeval tv{0, 0};
    fd_set rfds;
    FD_ZERO(&rfds);
    FD_SET(STDIN_FILENO, &rfds);
    if (select(STDIN_FILENO + 1, &rfds, nullptr, nullptr, &tv) > 0) {
      ssize_t n = ::read(STDIN_FILENO, &key, 1);
      if (n <= 0) key = 0;
    }

    if (key == 'q' || key == 'Q') running = false;
    if (key == 'o' || key == 'O') do_open();
    if (key == 's' || key == 'S') do_snap();
    if (key == 'c' || key == 'C') do_close();
    if (key == 'w' || key == 'W') enter_wifi();
    if (key == 'g' || key == 'G') enter_gallery();
    if (key == 'v' || key == 'V') enter_video_gallery();
    if (key == 'r' || key == 'R') toggle_record();

    if (tapped) {
      if (view == View::Main && gallery_top_button(sw).contains(sx, sy)) {
        enter_gallery();
        continue;
      }
      if (view == View::Main && wifi_top_button(sw).contains(sx, sy)) {
        enter_wifi();
        continue;
      }
      if (view == View::Gallery) {
        Button photo_tab{"PHOTO", sw - 286, 18, 126, 44, gallery_video_tab ? C_LEATHER : C_ACCENT};
        Button video_tab{"VIDEO", sw - 150, 18, 126, 44, gallery_video_tab ? C_ACCENT : C_LEATHER};
        if (photo_tab.contains(sx, sy)) {
          gallery_video_tab = false;
          video_playing = false;
          refresh_gallery();
          gallery_message = gallery_items.empty() ? "NO IMAGES" : "CAPTURED IMAGES";
          draw_current_gallery();
        } else if (video_tab.contains(sx, sy)) {
          enter_video_gallery();
        } else if (!gallery_video_tab) {
          if (gallery_buttons[0].contains(sx, sy)) {
            enter_main();
          } else if (gallery_buttons[1].contains(sx, sy)) {
            refresh_gallery();
            if (!gallery_items.empty()) {
              gallery_index = (gallery_index + static_cast<int>(gallery_items.size()) - 1) %
                              static_cast<int>(gallery_items.size());
              gallery_message = "PREVIOUS";
            }
            draw_current_gallery();
          } else if (gallery_buttons[2].contains(sx, sy)) {
            refresh_gallery();
            if (!gallery_items.empty()) {
              gallery_index = (gallery_index + 1) % static_cast<int>(gallery_items.size());
              gallery_message = "NEXT";
            }
            draw_current_gallery();
          } else if (gallery_buttons[3].contains(sx, sy)) {
            refresh_gallery();
            if (!gallery_items.empty()) {
              std::string deleted = gallery_items[gallery_index].name;
              fs::remove(gallery_items[gallery_index].path);
              gallery_message = "DELETED " + clip_text(deleted, 32);
              refresh_gallery();
            } else {
              gallery_message = "NO IMAGE TO DELETE";
            }
            draw_current_gallery();
          } else if (swipe) {
            refresh_gallery();
            if (!gallery_items.empty()) {
              if (ex < sx) gallery_index = (gallery_index + 1) % static_cast<int>(gallery_items.size());
              else gallery_index = (gallery_index + static_cast<int>(gallery_items.size()) - 1) %
                                   static_cast<int>(gallery_items.size());
              gallery_message = "SWIPE";
            }
            draw_current_gallery();
          }
        } else {
          if (video_buttons[0].contains(sx, sy)) {
            video_playing = false;
            enter_main();
          } else if (video_buttons[1].contains(sx, sy)) {
            refresh_video_sessions();
            if (!video_sessions.empty()) {
              video_index = (video_index + static_cast<int>(video_sessions.size()) - 1) %
                            static_cast<int>(video_sessions.size());
              video_frames = load_video_frames(video_sessions[video_index].dir);
              video_frame_index = 0;
              video_playing = false;
              record_message = "PREVIOUS VIDEO";
            }
            draw_current_gallery();
          } else if (video_buttons[2].contains(sx, sy)) {
            refresh_video_sessions();
            if (!video_sessions.empty()) {
              video_index = (video_index + 1) % static_cast<int>(video_sessions.size());
              video_frames = load_video_frames(video_sessions[video_index].dir);
              video_frame_index = 0;
              video_playing = false;
              record_message = "NEXT VIDEO";
            }
            draw_current_gallery();
          } else if (video_buttons[3].contains(sx, sy) ||
                     (sx > sw / 2 - 58 && sx < sw / 2 + 58 && sy > sh / 2 - 58 && sy < sh / 2 + 58)) {
            if (!video_sessions.empty() && !video_frames.empty()) {
              video_playing = !video_playing;
              last_video_frame_ms = monotonic_ms();
              record_message = video_playing ? "PLAYING" : "PAUSED";
            } else {
              record_message = "NO VIDEO TO PLAY";
            }
            draw_current_gallery();
          } else if (swipe) {
            refresh_video_sessions();
            if (!video_sessions.empty()) {
              if (ex < sx) video_index = (video_index + 1) % static_cast<int>(video_sessions.size());
              else video_index = (video_index + static_cast<int>(video_sessions.size()) - 1) %
                                  static_cast<int>(video_sessions.size());
              video_frames = load_video_frames(video_sessions[video_index].dir);
              video_frame_index = 0;
              video_playing = false;
              record_message = "SWIPE VIDEO";
            }
            draw_current_gallery();
          }
        }
        continue;
      }
      if (view == View::WifiList) {
        if (wifi_list_buttons[0].contains(sx, sy)) {
          enter_main();
        } else if (wifi_list_buttons[1].contains(sx, sy)) {
          do_scan_wifi();
        } else if (wifi_list_buttons[2].contains(sx, sy)) {
          wifi_message = "IP " + current_ip();
          if (has_fb) draw_wifi_list(fb, wifi_networks, wifi_message, wifi_list_buttons);
        } else {
          int row_h = 42;
          int index = (sy - 142) / row_h;
          if (sx >= 32 && sx <= sw - 32 && sy >= 142 &&
              index >= 0 && index < static_cast<int>(wifi_networks.size())) {
            selected_ssid = wifi_networks[index].ssid;
            wifi_password.clear();
            wifi_message = "ENTER PASSWORD";
            view = View::WifiPassword;
            if (has_fb) draw_wifi_password(fb, selected_ssid, wifi_password, wifi_upper, wifi_message, wifi_pass_buttons);
          }
        }
        continue;
      }
      if (view == View::WifiPassword) {
        if (wifi_pass_buttons[0].contains(sx, sy)) {
          view = View::WifiList;
          if (has_fb) draw_wifi_list(fb, wifi_networks, wifi_message, wifi_list_buttons);
        } else if (wifi_pass_buttons[1].contains(sx, sy)) {
          if (!wifi_password.empty()) wifi_password.pop_back();
          if (has_fb) draw_wifi_password(fb, selected_ssid, wifi_password, wifi_upper, wifi_message, wifi_pass_buttons);
        } else if (wifi_pass_buttons[2].contains(sx, sy)) {
          wifi_upper = !wifi_upper;
          if (has_fb) draw_wifi_password(fb, selected_ssid, wifi_password, wifi_upper, wifi_message, wifi_pass_buttons);
        } else if (wifi_pass_buttons[3].contains(sx, sy)) {
          wifi_message = "CONNECTING...";
          if (has_fb) draw_wifi_password(fb, selected_ssid, wifi_password, wifi_upper, wifi_message, wifi_pass_buttons);
          bool ok = connect_wifi(selected_ssid, wifi_password);
          wifi_message = ok ? ("CONNECTED IP " + current_ip()) : "CONNECT FAILED";
          view = View::WifiList;
          if (has_fb) draw_wifi_list(fb, wifi_networks, wifi_message, wifi_list_buttons);
        } else {
          const std::string keys = wifi_upper ? "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-" :
                                                "abcdefghijklmnopqrstuvwxyz0123456789._-";
          int cols = 10;
          int cell_w = (sw - 64) / cols;
          int cell_h = 42;
          int start_x = 32;
          int start_y = 158;
          if (sx >= start_x && sy >= start_y) {
            int col = (sx - start_x) / cell_w;
            int row = (sy - start_y) / cell_h;
            int idx = row * cols + col;
            if (col >= 0 && col < cols && idx >= 0 && idx < static_cast<int>(keys.size()) &&
                wifi_password.size() < 63) {
              wifi_password.push_back(keys[idx]);
              if (has_fb) draw_wifi_password(fb, selected_ssid, wifi_password, wifi_upper, wifi_message, wifi_pass_buttons);
            }
          }
        }
        continue;
      }
      if (swipe) {
        status = ex > sx ? "SWIPE RIGHT" : "SWIPE LEFT";
        main_shell_dirty = true;
        draw_main_shell();
        continue;
      }
      if (main_left_buttons[0].contains(sx, sy)) do_open();
      else if (main_left_buttons[1].contains(sx, sy)) do_snap();
      else if (main_left_buttons[2].contains(sx, sy)) do_close();
      else if (main_left_buttons[3].contains(sx, sy)) toggle_record();
    }

    if (view == View::Gallery && gallery_video_tab && video_playing && !video_frames.empty()) {
      int64_t now_ms = monotonic_ms();
      if (now_ms - last_video_frame_ms >= 200) {
        last_video_frame_ms = now_ms;
        video_frame_index++;
        if (video_frame_index >= static_cast<int>(video_frames.size())) {
          video_frame_index = static_cast<int>(video_frames.size()) - 1;
          video_playing = false;
          record_message = "PLAY DONE";
        }
        draw_current_gallery();
      }
    }

    if (view == View::Main && cam.is_open()) {
      Frame frame;
      if (cam.capture(frame, 600) && !frame.rgb.empty()) {
        if (yolo_ready && (frame_counter % 5 == 0)) {
          detections = yolo.infer(frame.rgb, frame.width, frame.height);
          summarize_detections();
          status = detections.empty() ? "LIVE YOLO" : "DETECTED";
        }
        if (record_enabled) {
          recorder.record(frame.rgb, frame.width, frame.height, detections, monotonic_ms());
          record_message = "REC " + std::to_string(recorder.frame_count);
        }
        if (has_fb) {
          if (main_shell_dirty) draw_main_shell();
          int left_x = 20;
          int left_y = 96;
          int left_w = std::max(220, fb.width() / 4);
          int right_x = left_x + left_w + 18;
          int right_y = 96;
          int right_w = fb.width() - right_x - 20;
          int right_h = fb.height() - 118;
          int preview_x = right_x + 12;
          int preview_y = right_y + 12;
          int preview_w = right_w - 24;
          int preview_h = right_h - 24;
          fb.draw_rgb(preview_x, preview_y, preview_w, preview_h, frame.rgb, frame.width, frame.height);
          draw_detections(fb, detections, preview_x, preview_y, preview_w, preview_h, frame.width, frame.height);
          fb.fill_rect(preview_x + 10, preview_y + 10, std::min(preview_w - 20, 520), 56,
                       blend_rgb(C_BG, C_LEATHER, 0.40f));
          fb.draw_rect(preview_x + 10, preview_y + 10, std::min(preview_w - 20, 520), 56, C_LEATHER);
          fb.draw_text(preview_x + 16, preview_y + 18, "STATUS: " + status, C_TEXT, 2);
          fb.draw_text(preview_x + 16, preview_y + 44, "RESULT: " + result, C_ACCENT, 2);
          fb.flush();
        }
        frame_counter++;
      }
    }
  }

  cam.close();
  if (has_fb) {
    fb.clear(0x000000);
    fb.draw_text(20, 20, "EXIT", C_TEXT, 3);
  }
  return 0;
}
