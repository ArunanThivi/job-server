FROM alpine:3.21

WORKDIR /app
COPY server .

RUN chmod +x ./server

EXPOSE 5000
ENTRYPOINT [ "./server" ]