# Multi-stage build for SDT web interface
# ref. https://lipanski.com/posts/smallest-docker-image-static-website

FROM lipanski/docker-static-website:2.4.0

# Copy the built web distribution
COPY ./dist /home/static

# Switch to non-root user for security
USER appuser

# Expose port 3000
EXPOSE 3000

# Start the HTTP server
CMD ["/busybox-httpd", "-f", "-v", "-p", "3000", "-c", "httpd.conf"]
