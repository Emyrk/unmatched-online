FROM golang:1.14

# Get git
RUN apt-get update \
    && apt-get -y install apt-utils curl git \
    && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Where factom-walletd sources will live
WORKDIR $GOPATH/src/github.com/Emyrk/unmatched-online

# Populate the rest of the source
COPY . .


# Build and install unmatched-online
RUN go install

ENTRYPOINT ["/go/bin/unmatched-online"]

EXPOSE 1111
