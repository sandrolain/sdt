# ref. https://lipanski.com/posts/smallest-docker-image-static-website

FROM lipanski/docker-static-website:2.4.0

COPY ./web/dist .

USER appuser

CMD ["/busybox-httpd", "-f", "-v", "-p", "3000", "-c", "httpd.conf"]
