# The release image: the server and Debian's Chromium, so an agent needs only
# Docker. GoReleaser builds it once per platform from a context that holds
# each platform's binary under $TARGETPLATFORM; `make image` copies that layout.
FROM debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad

# Debian's chromium pulls in no fonts, and pages without them screenshot as
# boxes. tini reaps the renderers Chromium orphans, which the server, as PID 1,
# would leave as zombies.
RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		ca-certificates chromium fonts-liberation fonts-noto-color-emoji tini \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd --create-home --uid 10001 browser

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/usechrome-mcp /usr/local/bin/usechrome-mcp

USER browser
# Docker's default seccomp profile denies the user namespaces Chromium's
# sandbox needs, so the container is the sandbox.
ENTRYPOINT ["tini", "--", "usechrome-mcp", "-no-sandbox"]
