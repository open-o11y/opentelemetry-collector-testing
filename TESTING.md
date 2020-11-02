# Testing

This repo is specifically setup to test the Prometheus pipeline. The cortex exporter is a component that has been added to this repo, and contains 
AWS Sigv4 logic combined with the Prometheus Remote Write Exporter. 

This testing setup involves kubernetes, so it is assumed you have set up an Amazon EKS cluster and have an ECR repo to host containers

In order to set up the collector, run `make docker-otelcol` to build the docker image

Push the docker image to your ECR repo

Make necessary changes to the file `docker-config.yaml` in the root directory of this repo,
and apply using `kubectl apply -f docker-config.yaml`

You should now have the collector set up in a k8s pod and exporting prometheus metrics to a cortex instance