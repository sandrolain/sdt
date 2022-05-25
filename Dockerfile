# ref. https://lipanski.com/posts/smallest-docker-image-static-website

FROM lipanski/docker-static-website:latest

COPY ./web/dist .

CMD ["/thttpd", "-D", "-h", "0.0.0.0", "-p", "3000", "-d", "/home/static", "-u", "static", "-l", "-", "-M", "60"]
