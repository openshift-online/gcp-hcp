# Static SPA served by a UBI nginx (per gcp-hcp-infra's ubi-minimal pattern).
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:08d4c70f1baf6537a0728656cf7942d561e74a35916509d205b6519861266352 AS build

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:08d4c70f1baf6537a0728656cf7942d561e74a35916509d205b6519861266352

LABEL org.opencontainers.image.title="marketplace-signup-frontend" \
      org.opencontainers.image.description="Interim marketplace sign-up SPA (experiment)" \
      org.opencontainers.image.source="https://github.com/openshift-online/gcp-hcp" \
      org.opencontainers.image.vendor="Red Hat" \
      vendor="Red Hat, Inc." \
      com.redhat.component="marketplace-signup-frontend" \
      summary="Marketplace sign-up SPA" \
      distribution-scope="private"

RUN microdnf install -y --nodocs nginx && microdnf clean all && rm -rf /var/cache/dnf /var/cache/yum

# nginx config: listen on 8080 (unprivileged), SPA fallback
COPY nginx.conf /etc/nginx/nginx.conf
# Static assets staged under /etc/signup; entrypoint renders to /usr/share/nginx/html (tmpfs)
COPY index.html app.js styles.css /etc/signup/
COPY --chmod=0755 40-signup-config.sh /docker-entrypoint.d/40-signup-config.sh

RUN mkdir -p /usr/share/nginx/html /var/cache/nginx /var/log/nginx /run && \
    chown -R 1001:0 /usr/share/nginx /var/cache/nginx /var/log/nginx /run /etc/nginx && \
    chmod -R g+rwX /usr/share/nginx /var/cache/nginx /var/log/nginx /run

USER 1001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD curl -f http://localhost:8080/index.html || exit 1
ENTRYPOINT ["/docker-entrypoint.d/40-signup-config.sh"]
CMD ["nginx", "-g", "daemon off;"]
