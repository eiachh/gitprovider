# Git Provider Server

A lightweight Git backend server with a minimal REST API for repository discovery and tree inspection.

## Features

- Accepts `git push` over HTTP and stores Git objects under `./data/<repo>`
- REST API for repository discovery and tree inspection
- OpenAPI document served at `/`
- Gin request logging enabled (standard Gin logger)

## REST API

All API responses are JSON unless explicitly noted below.

### OpenAPI

- `GET /`
  - Returns a minimal OpenAPI document describing the REST endpoints.

### Repositories

- `GET /repos`
  - Lists all repositories.

- `GET /repos/{repo}`
  - Returns repository metadata including default branch and available branches.

### Tree Inspection

- `GET /repos/{repo}/tree/{branch}`
  - Returns top-level entries (files/directories) for the branch.

- `GET /repos/{repo}/tree/{branch}/{path}`
  - If `{path}` is a directory: returns entries for that directory.
  - If `{path}` is a file: returns the raw file content (non-JSON).

## Git Protocol

The Git client endpoints are handled separately and are not included in the OpenAPI document:

- `GET /{repo}.git/info/refs?service=git-receive-pack`
- `POST /{repo}.git/git-receive-pack`

## Usage

1. Start the server:
   - Build and run: `go build -o server . && ./server`
   - The server binds to a dynamic port by default.
   - You can set a specific port with `GIT_SERVER_PORT`.

2. Push a repository:
   - `git remote add origin http://localhost:<port>/<repo>.git`
   - `git push -u origin <branch>`

3. Explore the API:
   - `GET /` for the OpenAPI document
   - `GET /repos` to list repositories
   - `GET /repos/{repo}` for branches
   - `GET /repos/{repo}/tree/{branch}` for tree entries
   - `GET /repos/{repo}/tree/{branch}/{path}` for a subtree or file content
