package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var pushImagesCmd = &cobra.Command{
	Use:   "pushimages",
	Short: "pushimages can be used to populate a registry with images from a text file",
	Run:   runPushImages,
}

var (
	imageFile    string
	registryURL  string
	registryUser string
	registryPass string
)

func init() {
	pushImagesCmd.PersistentFlags().StringVarP(&imageFile, "image-file", "f", "", "File containing newline separated images to push to registry")
	pushImagesCmd.PersistentFlags().StringVarP(&registryURL, "registry-url", "r", "", "Image registry url")
	pushImagesCmd.PersistentFlags().StringVar(&registryUser, "user", "", "Private registry user")
	pushImagesCmd.PersistentFlags().StringVar(&registryPass, "pass", "", "Private registry password")
}

const (
	numPushWorkers = 1
)

func runPushImages(cmd *cobra.Command, args []string) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Debug("running pushimages command")

	var err error
	imageFile, err = filepath.Abs(imageFile)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Debugf("pushing images in file: %v", imageFile)

	f, err := os.Open(imageFile)
	if err != nil {
		logrus.Fatalf("could not open image file: %v", err)
	}

	fbytes, err := ioutil.ReadAll(f)
	if err != nil {
		logrus.Fatalf("could not read contents of: %v, %v", imageFile, err)
	}

	images := strings.Split(string(fbytes), "\n")
	logrus.Debugf("images: %v", images)

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithTimeout(10*time.Minute),
	)
	if err != nil {
		panic(err)
	}

	imageC := make(chan string, numPushWorkers)
	go func() {
		defer close(imageC)
		for _, image := range images {
			imageC <- image
		}
	}()

	var auth string
	if registryUser != "" && registryPass != "" {
		logrus.Debug("private registry detected, logging in")

		var authConfig = types.AuthConfig{
			Username:      registryUser,
			Password:      registryPass,
			ServerAddress: registryURL,
		}

		authBytes, _ := json.Marshal(authConfig)
		auth = base64.URLEncoding.EncodeToString(authBytes)
	}

	var wg sync.WaitGroup
	wg.Add(numPushWorkers)
	for i := 1; i <= numPushWorkers; i++ {
		go func(w int) {
			defer wg.Done()
			logrus.Debugf("starting worker %v", w)
			for image := range imageC {
				rcls, err := cli.ImagePull(context.Background(), image, types.ImagePullOptions{})
				if err != nil {
					logrus.Errorf("pull error: %v", err)
					continue
				}

				err = rcls.Close()
				if err != nil {
					logrus.Errorf("i/o error: %v", err)
				}

				imageTarget := strings.Join([]string{registryURL, image}, "/")
				logrus.Debugf("source=%v target=%v", image, imageTarget)
				err = cli.ImageTag(context.Background(), image, imageTarget)
				if err != nil {
					logrus.Errorf("tag error: %v", err)
					continue
				}

				rcls, err = cli.ImagePush(context.Background(), imageTarget, types.ImagePushOptions{RegistryAuth: auth})
				if err != nil {
					logrus.Errorf("push error: %v", err)
					continue
				}

				res, err := ioutil.ReadAll(rcls)
				if err != nil {
					logrus.Error("i/o error: %v", err)
					continue
				}

				logrus.Debugf("successful push, image=%v res=%v", imageTarget, string(res))

				err = rcls.Close()
				if err != nil {
					logrus.Errorf("i/o error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}
