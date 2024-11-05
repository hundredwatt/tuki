# Kamal

```yaml
accessories:
  tuki:
    image: registry-76.localcan.dev/tuki:latest
    host: 192.168.72.2
    env:
      clear:
        # https://docs.github.com/en/authentication/connecting-to-github-with-ssh/using-ssh-agent-forwarding
        REPO_URL: git@github.com...
        SSH_AUTH_SOCK: /ssh-agent/ssh-agent.socket
    volumes:
      # ssh from host 
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "/run/user/1000:/ssh-agent:ro"
      - "/home/jason/.ssh/known_hosts:/root/.ssh/known_hosts:ro"
      # if your harness needs Kamal env variables
      - "/home/jason/.kamal/:/root/.kamal:ro" 
```
