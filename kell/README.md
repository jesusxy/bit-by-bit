# Kell ☸️

Inspired by the concept of a caul, `kell` is a minimalist Kubernetes operator that acts as a thin, controlling layer between user intent and underlying cluster resources.

This project is an experiment in understanding the core reconciliation loop and control patterns that power Kubernetes automation, built from first principles.

## Features

* Defines a `StaticWebsite` Custom Resource for declaring a desired website deployment.
* Automatically creates a Kubernetes `Deployment` and `Service` for each `StaticWebsite` resource.
* Uses an `initContainer` to clone a public Git repository.
* Uses an `nginx` container to serve the cloned static content.
* Cleans up the `Deployment` and `Service` when the `StaticWebsite` resource is deleted.

## Getting Started

These instructions assume you have **Go**, **Docker Desktop**, and **Minikube** installed.

#### 1. Start your local cluster
```bash
minikube start
```

#### 2. Install the Custom Resource Definition (CRD)
This command teaches your cluster what a StaticWebsite is.

```bash
# From inside the kell/operator/ directory
make install
```

#### 3. Run the Operator
This command runs the controller locally. It will occupy the current terminal.
```bash
# From inside the kell/operator/ directory
make run
```

#### 4. Deploy a Website 
In a new terminal, create a YAML file for your website (e.g `my-first-website`)
```yaml
apiVersion: [webapp.com/v1alpha1](https://webapp.com/v1alpha1)
kind: StaticWebsite
metadata:
  name: my-first-website
spec:
  # Any public git repo with an index.html will work
  gitRepo: "[https://github.com/mdn/beginner-html-site-styled.git](https://github.com/mdn/beginner-html-site-styled.git)"
  replicas: 2
```
Apply the manifest to deploy the site:
```bash
# From the directory where your YAML file is located
kubectl apply -f my-first-website.yaml
```

#### 5. Access your website
User the `minikube` command to get the URL and open it in your browser.
```bash
minikube service my-first-website
```

## DEMO
