run: build
	./k8sdump

build: clean
	go build -o k8sdump main.go

clean:
	rm k8sdump
