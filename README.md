## Setup

The `server.yaml` file does quite a bit:

1) Sets up a deployment, pulling the latest version of the job-server, and creating an account with all the proper permissions we might need
2) Establishes a local container registry that we will later fill with images our autograder will use
3) A job to pull and cache commonly used images (namely `moby/buildkit` for building containers and `gradescope/autograder-base:ubuntu-22.04` as our base image)

**Note:** Because the registry needs to be accessed by both the pods and the kubernetes service itself, at LEAST one pod must be configured with a __static IP__. This static IP will be referred to as **$REGISTRY_IP**. 

For the kube service to access the local registry, an edit needs to be made in containerd. Specifically, The following snippet must be written at `/etc/containerd/certs.d/$REGISTRY_IP:31804/hosts.toml`
```
server = "http://$REGISTRY_IP:31804"

[host."http://$REGISTRY_IP:31804"]
  capabilities = ["pull", "push", "resolve"]
  plain-http = true
```

Build the binary with `GOOS=linux GOARM=arm64 go build -o server -ldflags="-s -w" server.go`

## API Reference

<details>
 <summary><code>GET</code> <code><b>/</b></code> <code>(Gets job-server version)</code></summary>

##### Parameters

> None

##### Responses

> | http code     | content-type                      | response                                                            |
> |---------------|-----------------------------------|---------------------------------------------------------------------|
> | `200`         | `text/plain;charset=UTF-8`        | YAML string                                                         |
> | `405`         | `text/html;charset=utf-8`         | "Only GET method allowed" |
##### Example cURL

> ```javascript
>  curl -X GET -H "Content-Type: application/json" http://localhost:5000/
> ```

</details>


<details>
 <summary><code>POST</code> <code><b>/configure</b></code> <code>(Configure custom autograder image)</code></summary>

##### Parameters

> | name      |  type     | data type               | description                                                           |
> |-----------|-----------|-------------------------|-----------------------------------------------------------------------|
> | setup     |  required | zip archive   | ZIP of all dependencies. Must include `setup.sh` and `run_autograder` in root of archive |
> | assignment     |  required | string   | Name of assignment to be graded |


##### Responses

> | http code     | content-type                      | response                                                            |
> |---------------|-----------------------------------|---------------------------------------------------------------------|
> | `201`         | `text/plain;charset=UTF-8`        | `Configuration created successfully`                                |
> | `405`         | `text/html;charset=utf-8`         | "Only POST method allowed"|
> | `400`         | `text/html;charset=utf-8` | "Missing `<parameter>` field" |
> | `500`         | `text/html;charset=utf-8` | "Failed to create runner Job" |
##### Example cURL

> ```javascript
>  curl -X POST -F "setup=@py_autograder.zip" -F "assignment=python" http://localhost:5000/configure  
> ```

</details>

<details>
 <summary><code>POST</code> <code><b>/submit</b></code> <code>(Submit student code to be graded)</code></summary>

##### Parameters

> | name      |  type     | data type               | description                                                           |
> |-----------|-----------|-------------------------|-----------------------------------------------------------------------|
> | assignment |  required | string | Name of assignment to be graded. Should match param passed into `/configure`  |
> | student |  required | string | Name of student that owns submission  |
> | submission |  required | zip | archive of code to be graded  |


##### Responses

> | http code     | content-type                      | response                                                            |
> |---------------|-----------------------------------|---------------------------------------------------------------------|
> | `201`         | `text/plain;charset=UTF-8` | `Configuration created successfully` |
> | `400`         | `text/html;charset=utf-8` | "Missing `<parameter>` field" |
> | `405`         | `text/html;charset=utf-8` | "Only POST method allowed" |
> | `500`         | `text/html;charset=utf-8` | "Failed to create runner Job" |

##### Example cURL

> ```javascript
>  curl -X POST -F "submission=@submission.zip" -F "assignment=python" -F "student=Arunan" http://localhost:5000/submit
> ```

</details>

<details>
 <summary><code>GET</code> <code><b>/results</b></code> <code>(Returns job status or autograde result)</code></summary>

##### Parameters

> | name      |  type     | data type               | description                                                           |
> |-----------|-----------|-------------------------|-----------------------------------------------------------------------|
> | assignment |  required | string | Name of assignment to be graded. Should match param passed into `/submit`  |
> | student |  required | string | Name of student that owns submission. Should match param passed into `/submit` |

##### Responses

> | http code     | content-type                      | response                                                            |
> |---------------|-----------------------------------|---------------------------------------------------------------------|
> | `200`         | `application/json`        | `{status: running}` OR `{status: failed}` OR `{autograde results}` |
> | `405`         | `text/html;charset=utf-8`         | "Only GET method allowed"|
> | `400`         | `text/html;charset=utf-8` | "Missing `<parameter>` field" |
> | `404`         | `text/html;charset=utf-8` | "Job not found" OR "no pods for job" |
> | `500`         | `text/html;charset=utf-8` | "Failed to get logs" OR "failed to read logs" |
##### Example cURL

> ```javascript
>  curl -X GET "http://localhost:5000/results?student=arunan&assignment=python"
> ```

</details>