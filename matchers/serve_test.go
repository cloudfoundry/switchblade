package matchers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudfoundry/switchblade"
	"github.com/cloudfoundry/switchblade/matchers"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testServe(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect  = NewWithT(t).Expect
		matcher *matchers.ServeMatcher
		server  *httptest.Server
	)

	it.Before(func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodHead {
				http.Error(w, "NotFound", http.StatusNotFound)
				return
			}

			switch req.URL.Path {
			case "/":
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "some string")
			case "/empty":
				// do nothing
			case "/teapot":
				w.WriteHeader(http.StatusTeapot)
				fmt.Fprint(w, "some string")
			default:
				fmt.Fprintln(w, "unknown path")
				t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
			}
		}))

		matcher = matchers.Serve("some string")
	})

	it.After(func() {
		server.Close()
	})

	context("Match", func() {
		context("the http request succeeds and response equals expected", func() {
			it("returns true", func() {
				result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		context("the http request succeeds and response matches expected", func() {
			it.Before(func() {
				matcher = matchers.Serve(ContainSubstring("me str"))
			})

			it("returns true", func() {
				result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		context("the actual response does not match expected response", func() {
			it.Before(func() {
				matcher = matchers.Serve("another string")
			})

			it("returns false", func() {
				result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		context("the http response is nil", func() {
			it.Before(func() {
				matcher = matcher.WithEndpoint("/empty")
			})

			it("returns false", func() {
				result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		context("the response status code is not OK", func() {
			it.Before(func() {
				matcher = matcher.WithEndpoint("/teapot")
			})

			it("returns false", func() {
				result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		context("failure cases", func() {
			context("when the matcher is not given a deployment", func() {
				it("returns an error", func() {
					result, err := matcher.Match("this url is not a deployment")
					Expect(err).To(MatchError("ServeMatcher expects a switchblade.Deployment, received string"))
					Expect(result).To(BeFalse())
				})
			})

			context("when the deployment URL cannot be parsed", func() {
				it("returns an error", func() {
					result, err := matcher.Match(switchblade.Deployment{ExternalURL: "%%%"})
					Expect(err).To(MatchError(ContainSubstring(`invalid URL escape "%%%"`)))
					Expect(result).To(BeFalse())
				})
			})

			context("when the request URL is malformed", func() {
				it("returns an error", func() {
					result, err := matcher.Match(switchblade.Deployment{ExternalURL: "this url is garbage"})
					Expect(err).To(MatchError(ContainSubstring("unsupported protocol scheme")))
					Expect(result).To(BeFalse())
				})
			})

			context("when the given matcher errors", func() {
				it.Before(func() {
					matcher = matchers.Serve(MatchRegexp(`(((`))
				})

				it("returns an error", func() {
					result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
					Expect(err).To(MatchError(ContainSubstring("error parsing regexp")))
					Expect(result).To(BeFalse())
				})
			})
		})
	})

	context("when the matcher fails", func() {
		it.Before(func() {
			matcher = matchers.Serve("no such content")
			result, err := matcher.Match(switchblade.Deployment{ExternalURL: server.URL})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		context("FailureMessage", func() {
			it("returns a useful error message", func() {
				message := matcher.FailureMessage("some string")
				Expect(message).To(ContainSubstring(strings.TrimSpace(`
Expected the response from deployment:

	some string

to contain:

	no such content`)))
			})
		})

		context("NegatedFailureMessage", func() {
			it("returns a useful error message", func() {
				message := matcher.NegatedFailureMessage("some string")
				Expect(message).To(ContainSubstring(strings.TrimSpace(`
Expected the response from deployment:

	some string

not to contain:

	no such content`)))
			})
		})
	})
}
