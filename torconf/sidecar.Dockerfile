# Use a lightweight base image
FROM alpine:latest

# Install Tor
RUN apk add --no-cache tor

# Entry point script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

# Make the script executable
RUN chmod +x /usr/local/bin/entrypoint.sh

# Set the entrypoint to the script
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
