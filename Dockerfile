FROM alpine:3.15

ADD dist.tar.gz /opt/app

ENTRYPOINT [ "/opt/app/stargazer" ]
WORKDIR "/opt/app" 

CMD ["web"]
