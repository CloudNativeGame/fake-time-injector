export GOPROXY  := $(if $(GOPROXY),$(GOPROXY),https://proxy.golang.org,direct)
GOENV   := CGO_ENABLED=0
CGOENV  := CGO_ENABLED=1
GO      := $(GOENV) go
CGO     := $(CGOENV) go
watchmaker: pkg/time/fakeclock/fake_clock_gettime.o pkg/time/fakeclock/fake_gettimeofday.o
		$(CGO) build -ldflags '$(LDFLAGS)' -o bin/watchmaker ./cmd/watchmaker/...
pkg/time/fakeclock/fake_clock_gettime.o:
		gcc -c pkg/time/fakeclock/fake_clock_gettime.c -fPIE -O2 -o pkg/time/fakeclock/fake_clock_gettime.o
pkg/time/fakeclock/fake_gettimeofday.o:
		gcc -c pkg/time/fakeclock/fake_gettimeofday.c -fPIE -O2 -o pkg/time/fakeclock/fake_gettimeofday.o
