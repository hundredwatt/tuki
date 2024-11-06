# Tuki Task Runner

<p align="center">
    <img src="imgs/tuki-logo.jpeg" width="200" height="200">
</p>

<p align="center">
    Safely Run Commands in Productions with Tuki!
</p>

<p align="center">
    <a href="https://github.com/hundredwatt/tuki">
        <img src="https://img.shields.io/badge/GitHub-100000?style=for-the-badge&logo=github&logoColor=white" alt="GitHub">
    </a>
</p>

Tuki provides an alternative to running commands in your production REPL or console directly. Instead, you can write your commands in a Git repository, use the standard git workflow to review and iterate on them, and then merge them into your production branch for execution.

## Features

- Execute scripts in your production environment from a Git repository.
- State is stored in the Git repository for persistence and visibility.
- Harness file for customizing how tasks are run.

## Project Status

Tuki is alpha software. We are actively looking for collaborators to work on this project with us. If you are interested, please reach out or submit a pull request!

# Quick Start

To use Tuki:

1. Create a new Github repository for your Tuki scripts.
2. Deploy Tuki to your production environment and configure it with the repo URL and other options.
3. Any scripts that are merged into the main branch will be run in the production environment.

# Deployment

Kamal can be deployed as a Go binary or as a Docker image.

## Kamal

To deploy Tuki with Kamal, first complete the prequisites:

1. Create a new Github repository for your Tuki scripts.
2. Configure a harness file in your repository at `.tuki/harness.sh`, here's a Rails/Kamal example:

```sh
#!/bin/sh

docker run -i --rm --network kamal --env-file $KAMAL_ROLE_ENV_FILE my-rails-app:latest bin/rails runner -
```

3. Generate a new SSH key on your server and add it as a Deploy Key to your Github repository.
4. Enable SSH agent as a daemon on your server:

```
# /etc/systemd/system/ssh-agent.service
[Unit]
Description=SSH Agent

[Service]
Environment=SSH_AUTH_SOCK=%t/ssh-agent.socket
ExecStart=/usr/bin/ssh-agent -D -a $SSH_AUTH_SOCK

[Install]
WantedBy=default.target
```

```sh
sudo systemctl enable ssh-agent
sudo systemctl start ssh-agent
```

5. Verify your server has access to the Github repository and adds Github to its known hosts file by running `ssh -T git@github.com` (get help [here](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/using-ssh-agent-forwarding)).

6. Configure an accessory in your `config/deploy.yml` file and boot it:

```yaml
accessories:
  tuki:
    image: hundredwatt/tuki:latest
    host: xxx.xxx.xxx.xxx # replace with your server IP
    env:
      clear:
        REPO_URL: git@github.com... # replace with your scripts repo URL
        SSH_AUTH_SOCK: /ssh-agent/ssh-agent.socket
    volumes:
      # Share ssh agent socket and known hosts file with the container
      - "/run/user/$UID:/ssh-agent:ro"
      - "/home/$USER/.ssh/known_hosts:/root/.ssh/known_hosts:ro"
      # If your harness needs Docker access to launch other containers
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      # if your harness needs Kamal env variables
      - "/home/$USER/.kamal/:/root/.kamal:ro" 
```

```sh
kamal accessory boot tuki
```

7. Any scripts that are merged into the main branch will be run in the production environment!

# Configuration

## Harness File

The harness file is a shell script that is used to run tasks. It's stored in the repository at `.tuki/harness.sh`.

Each time a script is run, Tuki will run the harness file with the script contents as stdin.

### Postgres Example

For example, if you wanted to run SQL scripts on Postgres, you could create a harness file that looks like this:

```sh
#!/bin/sh

psql -d $DATABASE_URL
```

Then you could create a task in the repository that looks like this:

```sql
-- my-task.sql
UPDATE users SET name = 'Tuki' WHERE id = 1;
```

### Rails with Docker Example

Another example harness file could be one that runs a Rails console script via docker:

```sh
#!/bin/sh

docker run -i --rm --network kamal --env-file $KAMAL_ROLE_ENV_FILE my-rails-app:latest bin/rails runner -
```

Then you could create a task in the repository that looks like this:

```ruby
# my-task.rb
User.first.update(name: 'Tuki')
```

## Environment Variables

Tuki is configured using environment variables. Below are the key configuration options:

- `REPO_URL`: URL of the Git repository containing the scripts.
- `TICK_INTERVAL_SECONDS`: Interval between checking for new scripts to run (defaults to 60 seconds).
- `SCRIPTS_DIR`: Directory within the repository where scripts are located (defaults to `/`).
- `VERBOSE`: Enable verbose logging when set to `true` (defaults to `false`).


## State File

Tuki uses a state file to track which scripts have been run. It's stored in the repository at `.tuki/state.json`. Make sure your deploy keys have write access to the origin repository so updates to the state file can be pushed back to the repository for persistence and visibility.