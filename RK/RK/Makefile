CROSS_COMPILE ?= aarch64-linux-gnu-
CC := $(CROSS_COMPILE)gcc
CFLAGS ?= -O2 -Wall -Wextra -std=c11
LDFLAGS ?=
STATIC ?= 1

ROOT := $(CURDIR)
SRCDIR := $(ROOT)/src
OUTDIR := $(ROOT)/output

ifeq ($(STATIC),1)
LDFLAGS += -static
endif

TARGETS := main touch_test camera_capture

all: $(addprefix $(OUTDIR)/,$(TARGETS))

$(OUTDIR):
	mkdir -p $(OUTDIR)

$(OUTDIR)/main: $(SRCDIR)/main.c | $(OUTDIR)
	$(CC) $(CFLAGS) $< -o $@ $(LDFLAGS)

$(OUTDIR)/touch_test: $(SRCDIR)/touch_test.c | $(OUTDIR)
	$(CC) $(CFLAGS) $< -o $@ $(LDFLAGS)

$(OUTDIR)/camera_capture: $(SRCDIR)/camera_capture.c | $(OUTDIR)
	$(CC) $(CFLAGS) $< -o $@ $(LDFLAGS)

clean:
	rm -f $(addprefix $(OUTDIR)/,$(TARGETS))

.PHONY: all clean
