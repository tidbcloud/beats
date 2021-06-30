docker run --rm -it \
-v $(pwd)/deploy/localtest:/app -w /app \
docker.elastic.co/beats/filebeat:7.13.2 \
-c 'stdin-to-stdout.yaml' \
-e
