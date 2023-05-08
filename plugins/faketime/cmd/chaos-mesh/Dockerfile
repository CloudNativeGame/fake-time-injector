FROM golang:latest
RUN mkdir -p /opt/app
WORKDIR /opt/app
COPY . .
RUN make watchmaker
CMD ./start.sh
