FROM registry.apps.gammazeta.net/ghetzel/qt:arm
MAINTAINER Gary Hetzel <its@gary.cool>

COPY bin/hydra-linux-arm /usr/bin/hydra

EXPOSE 11647
CMD ["/usr/bin/hydra", "-L", "debug", "--server", "--run"]
