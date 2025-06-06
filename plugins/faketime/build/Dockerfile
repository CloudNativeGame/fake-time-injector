FROM golang:1.18
RUN apt-get update && apt-get upgrade -y && rm -rf /var/cache/apt/
RUN mkdir -p /opt/app
WORKDIR /opt/app/
ADD https://github.com/chaos-mesh/chaos-mesh/archive/refs/tags/v2.6.0.tar.gz /opt/app/v2.6.0.tar.gz
RUN tar -xvf /opt/app/v2.6.0.tar.gz -C /opt/app
WORKDIR /opt/app/chaos-mesh-2.6.0
ADD . .
RUN make watchmaker
RUN chmod +777 ./start.sh
CMD ./start.sh && sleep infinity