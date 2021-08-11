package docker_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuildpacksRegistry(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		registry docker.BuildpacksRegistry
		server   *httptest.Server
	)

	it.Before(func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("Authorization") != "Bearer some-token" {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			matches := regexp.MustCompile(`\/repos\/cloudfoundry\/(.*)\/releases\/latest`).FindStringSubmatch(req.URL.Path)
			if len(matches) != 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			name := strings.TrimSuffix(matches[1], "-buildpack")
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"assets": []map[string]interface{}{
					{
						"name":                 "some-file",
						"browser_download_url": "some-file-uri",
					},
					{
						"name":                 "other-file",
						"browser_download_url": "other-file-uri",
					},
					{
						"name":                 fmt.Sprintf("%s-buildpack.zip", name),
						"browser_download_url": fmt.Sprintf("some-%s-uri", name),
					},
				},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}))

		registry = docker.NewBuildpacksRegistry(server.URL, "some-token")
	})

	it.After(func() {
		server.Close()
	})

	context("List", func() {
		it("manages the canonical list of buildpacks", func() {
			list, err := registry.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(Equal([]docker.Buildpack{
				{
					Name: "staticfile_buildpack",
					URI:  "some-staticfile-uri",
				},
				{
					Name: "java_buildpack",
					URI:  "some-java-uri",
				},
				{
					Name: "ruby_buildpack",
					URI:  "some-ruby-uri",
				},
				{
					Name: "dotnet_core_buildpack",
					URI:  "some-dotnet-core-uri",
				},
				{
					Name: "nodejs_buildpack",
					URI:  "some-nodejs-uri",
				},
				{
					Name: "go_buildpack",
					URI:  "some-go-uri",
				},
				{
					Name: "python_buildpack",
					URI:  "some-python-uri",
				},
				{
					Name: "php_buildpack",
					URI:  "some-php-uri",
				},
				{
					Name: "nginx_buildpack",
					URI:  "some-nginx-uri",
				},
				{
					Name: "r_buildpack",
					URI:  "some-r-uri",
				},
				{
					Name: "binary_buildpack",
					URI:  "some-binary-uri",
				},
			}))
		})

		context("failure cases", func() {
			context("when the api is malformed", func() {
				it.Before(func() {
					registry = docker.NewBuildpacksRegistry("%%%", "some-token")
				})

				it("returns an error", func() {
					_, err := registry.List()
					Expect(err).To(MatchError(ContainSubstring("failed to create request:")))
					Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
				})
			})

			context("when the request fails", func() {
				it.Before(func() {
					registry = docker.NewBuildpacksRegistry("http://localhost:0", "some-token")
				})

				it("returns an error", func() {
					_, err := registry.List()
					Expect(err).To(MatchError(ContainSubstring("failed to complete request:")))
					Expect(err).To(MatchError(ContainSubstring("can't assign requested address")))
				})
			})

			context("whent the response status is not 200 OK", func() {
				it.Before(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprintf(w, "server encountered an error")
					}))

					registry = docker.NewBuildpacksRegistry(server.URL, "some-token")
				})

				it("returns an error", func() {
					_, err := registry.List()
					Expect(err).To(MatchError(ContainSubstring("received unexpected response status:")))
					Expect(err).To(MatchError(ContainSubstring("server encountered an error")))
				})
			})

			context("when the response json is malformed", func() {
				it.Before(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						fmt.Fprint(w, "%%%")
					}))

					registry = docker.NewBuildpacksRegistry(server.URL, "some-token")
				})

				it("returns an error", func() {
					_, err := registry.List()
					Expect(err).To(MatchError(ContainSubstring("failed to parse response json:")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})
		})
	})

	context("Override", func() {
		it("overrides the given buildpack", func() {
			registry.Override(docker.Buildpack{Name: "python_buildpack", URI: "override-python-uri"})
			list, err := registry.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(Equal([]docker.Buildpack{
				{
					Name: "staticfile_buildpack",
					URI:  "some-staticfile-uri",
				},
				{
					Name: "java_buildpack",
					URI:  "some-java-uri",
				},
				{
					Name: "ruby_buildpack",
					URI:  "some-ruby-uri",
				},
				{
					Name: "dotnet_core_buildpack",
					URI:  "some-dotnet-core-uri",
				},
				{
					Name: "nodejs_buildpack",
					URI:  "some-nodejs-uri",
				},
				{
					Name: "go_buildpack",
					URI:  "some-go-uri",
				},
				{
					Name: "python_buildpack",
					URI:  "override-python-uri",
				},
				{
					Name: "php_buildpack",
					URI:  "some-php-uri",
				},
				{
					Name: "nginx_buildpack",
					URI:  "some-nginx-uri",
				},
				{
					Name: "r_buildpack",
					URI:  "some-r-uri",
				},
				{
					Name: "binary_buildpack",
					URI:  "some-binary-uri",
				},
			}))
		})

		context("when the buildpack is not in the default list", func() {
			it("adds the given buildpack", func() {
				registry.Override(docker.Buildpack{Name: "extra_buildpack", URI: "some-extra-uri"})
				list, err := registry.List()
				Expect(err).NotTo(HaveOccurred())
				Expect(list).To(Equal([]docker.Buildpack{
					{
						Name: "staticfile_buildpack",
						URI:  "some-staticfile-uri",
					},
					{
						Name: "java_buildpack",
						URI:  "some-java-uri",
					},
					{
						Name: "ruby_buildpack",
						URI:  "some-ruby-uri",
					},
					{
						Name: "dotnet_core_buildpack",
						URI:  "some-dotnet-core-uri",
					},
					{
						Name: "nodejs_buildpack",
						URI:  "some-nodejs-uri",
					},
					{
						Name: "go_buildpack",
						URI:  "some-go-uri",
					},
					{
						Name: "python_buildpack",
						URI:  "some-python-uri",
					},
					{
						Name: "php_buildpack",
						URI:  "some-php-uri",
					},
					{
						Name: "nginx_buildpack",
						URI:  "some-nginx-uri",
					},
					{
						Name: "r_buildpack",
						URI:  "some-r-uri",
					},
					{
						Name: "binary_buildpack",
						URI:  "some-binary-uri",
					},
					{
						Name: "extra_buildpack",
						URI:  "some-extra-uri",
					},
				}))
			})
		})
	})
}
