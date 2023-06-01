FROM golang:1.18
RUN mkdir -p /opt/app
WORKDIR /opt/app
ENV GOPROXY=https://goproxy.cn,direct
COPY . .

RUN make watchmaker
RUN chmod +777 ./start.sh

CMD ./start.sh