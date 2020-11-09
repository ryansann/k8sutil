build: clean
	go build -o k8sutil main.go

clean:
	rm k8sutil
