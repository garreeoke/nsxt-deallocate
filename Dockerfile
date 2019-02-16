FROM alpine:3.4

WORKDIR /apl-loc-deploy
#RUN mkdir -p /apl-loc-deploy/interviews
COPY artifacts/nsxt-deallocate-linux.tgz .
RUN tar xzvf ./nsxt-deallocate-linux.tgz

CMD ["./nsxt-deallocate"]