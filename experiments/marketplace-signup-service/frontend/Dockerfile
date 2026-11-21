# Static SPA served by an unprivileged nginx.
#
# Configuration (public values only) is injected at container start into the
# page's signup-config meta tag, so the image is environment-agnostic and no
# inline script (which a CSP would block) is needed.
FROM docker.io/nginxinc/nginx-unprivileged:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html app.js styles.css /etc/signup/
COPY 40-signup-config.sh /docker-entrypoint.d/40-signup-config.sh

USER root
RUN chmod +x /docker-entrypoint.d/40-signup-config.sh
USER 101

EXPOSE 8080
