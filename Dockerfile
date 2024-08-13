FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY . .

RUN go mod download

RUN go build -ldflags="-s -w" -o dist/htmlpdf .

FROM surnet/alpine-wkhtmltopdf:3.20.2-0.12.6-full AS wkhtmltopdf
FROM alpine:latest

# Install dependencies for wkhtmltopdf
RUN apk add --no-cache \
    libstdc++ \
    libx11 \
    libxrender \
    libxext \
    libssl3 \
    ca-certificates \
    fontconfig \
    freetype \
    ttf-dejavu \
    ttf-droid \
    ttf-freefont \
    ttf-liberation \
    # more fonts
  && apk add --no-cache --virtual .build-deps \
    msttcorefonts-installer \
  # Install microsoft fonts
  && update-ms-fonts \
  && fc-cache -f \
  # Clean up when done
  && rm -rf /tmp/*
# Copy wkhtmltopdf files from docker-wkhtmltopdf image
COPY --from=wkhtmltopdf /bin/wkhtmltopdf /bin/wkhtmltopdf
COPY --from=wkhtmltopdf /bin/wkhtmltoimage /bin/wkhtmltoimage
COPY --from=wkhtmltopdf /lib/libwkhtmltox* /lib/

WORKDIR /app

COPY --from=builder /build/dist/htmlpdf .
COPY --from=builder /build/public ./public

CMD ["/app/htmlpdf"]
