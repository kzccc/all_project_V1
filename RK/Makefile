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

TARGETS := main

all: $(OUTDIR)/main

$(OUTDIR):
	mkdir -p $(OUTDIR)

$(OUTDIR)/main: $(SRCDIR)/main.c | $(OUTDIR)
	$(CC) $(CFLAGS) $< -o $@ $(LDFLAGS)

clean:
	rm -f $(addprefix $(OUTDIR)/,$(TARGETS))

.PHONY: all clean
