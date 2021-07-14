package switchblade_test

import (
	"testing"

	"github.com/ryanmoran/switchblade"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testRandomName(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	it("generates a random name", func() {
		name, err := switchblade.RandomName()
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(MatchRegexp(`^switchblade\-[0123456789abcdefghjkmnpqrstvwxyz]{26}$`))
	})
}
