#  Docker_watch
FROM scratch

# Add executable
ADD docker_watch /

EXPOSE 9100
ENTRYPOINT [ "/docker_watch" ]
