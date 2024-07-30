# Specifies a parent image
FROM golang:bullseye
 
# Creates an app directory to hold your app’s source code
WORKDIR /app
 
# Copies everything from your root directory into /app
COPY . .

# Builds your app with optional configuration
RUN go build -o main
 
# Tells Docker which network port your container listens on
EXPOSE 8080

RUN chmod 755 main
 
# Specifies the executable command that runs when the container starts
CMD [ "./main" ]