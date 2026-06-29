CXX ?= g++
TARGET ?= rk356x_demo
PREVIEW_TARGET ?= camera_preview
BUILD_DIR ?= build
SRC := src/main.cpp
PREVIEW_SRC := src/camera_preview.cpp

CXXFLAGS ?= -O2 -Wall -Wextra -std=c++17
CPPFLAGS ?= -Ithird_party/rknn
LDFLAGS ?= -Llib -Wl,-rpath,/root/rk356x_demo/lib -static-libstdc++ -static-libgcc
LDLIBS ?= -lrknnrt

.PHONY: all clean run probe

all: $(BUILD_DIR)/$(TARGET) $(BUILD_DIR)/$(PREVIEW_TARGET)

$(BUILD_DIR)/$(TARGET): $(SRC)
	mkdir -p $(BUILD_DIR)
	$(CXX) $(CPPFLAGS) $(CXXFLAGS) $< -o $@ $(LDFLAGS) $(LDLIBS)

$(BUILD_DIR)/$(PREVIEW_TARGET): $(PREVIEW_SRC)
	mkdir -p $(BUILD_DIR)
	$(CXX) $(CPPFLAGS) $(CXXFLAGS) $< -o $@ -static-libstdc++ -static-libgcc

run: all
	sudo ./$(BUILD_DIR)/$(TARGET)

probe:
	sh scripts/probe_board.sh

clean:
	rm -rf $(BUILD_DIR)
