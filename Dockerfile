FROM registry.apps.gammazeta.net/ghetzel/qt:arm
MAINTAINER Gary Hetzel <its@gary.cool>

ADD "https://www.random.org/cgi-bin/randbyte?nbytes=10&format=h" /.randomfriend
COPY bin/hydra-linux-arm /usr/bin/hydra

EXPOSE 11647
CMD ["/usr/bin/hydra", "-L", "debug", "--server", "--run"]
