FROM scratch
MAINTAINER Josip Janzic <josip@jjanzic.com>
ADD puull puull
ENV ROOT_URL https://puull.pw
ENV PORT 80
EXPOSE 80
ENTRYPOINT ["/puull"]
