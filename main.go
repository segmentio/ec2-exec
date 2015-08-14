package main

import "github.com/mattrobenolt/semaphore"
import "github.com/mitchellh/goamz/aws"
import "github.com/visionmedia/docopt"
import "github.com/segmentio/go-ec2"
import "github.com/segmentio/go-log"
import "strconv"
import "os/exec"
import "sync"
import "fmt"
import "os"

var Version = "0.1.1"

const Usage = `
  Usage:
    ec2-exec [--concurrency n] [--region name] [--name pattern] <cmd>...
    ec2-exec -h | --help
    ec2-exec --version

  Options:
    -n, --name pattern   filter hosts by name [default: *]
    -r, --region name    aws region [default: us-west-2]
    -c, --concurrency n  set concurrency [default: 1]
    -h, --help           output help information
    -v, --version        output version

`

func main() {
	args, err := docopt.Parse(Usage, nil, true, Version, false)
	log.Check(err)

	cmd := args["<cmd>"].([]string)
	name := args["--name"].(string)
	n, err := strconv.Atoi(args["--concurrency"].(string))
	log.Check(err)

	sem := semaphore.New(n)

	auth, err := aws.EnvAuth()
	log.Check(err)

	region, ok := aws.Regions[args["--region"].(string)]
	if !ok {
		log.Check(fmt.Errorf("invalid region name"))
	}

	client := ec2.New(auth, region)

	nodes, err := client.Name(name)
	log.Check(err)

	if len(nodes) == 0 {
		log.Error("no nodes matching %s", name)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	for _, node := range nodes {
		sem.Wait()
		wg.Add(1)

		go func(node ec2.Instance) {
			l := log.New(os.Stderr, log.INFO, node.Name())
			defer wg.Done()
			defer sem.Signal()
			args := []string{node.Name()}
			args = append(args, cmd...)
			c := exec.Command("ssh", args...)
			c.Stderr = l
			c.Stdout = l
			err := c.Run()
			if err != nil {
				l.Error("failed: %s", err)
			}
		}(node)
	}

	wg.Wait()
}
