BIN_NAME=my_sns_tool

build-client:
	go build -o $(BIN_NAME)-client -ldflags="-X main.mode=client" .

build-server:
	go build -o $(BIN_NAME)-server -ldflags="-X main.mode=server" .

build-all: build-client build-server

run:
	./$(BIN_NAME)-$(MODE)

clean:
	rm -f $(BIN_NAME)-client $(BIN_NAME)-server
