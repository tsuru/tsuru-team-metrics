# tsuru-team-metrics

Single endpoint to export some tsuru metadata as a prometheus metrics


# Run local

1. Build the go binary with:

    go build .

2. Have logged on tsuru once

    tsuru target-set -s default https://yourtsuruserver
    tsuru login

3. Run binary

   ./tsuru-team-metrics

4. Access metrics

    curl localhost:19283/metrics
