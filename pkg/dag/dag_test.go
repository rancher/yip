package dag_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/rancher/yip/pkg/dag"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dag Test Suite")
}

var _ = Describe("dag", func() {
	var g *Graph

	BeforeEach(func() {
		g = DAG()
	})

	Context("simple checks", func() {
		It("orders", func() {
			g.DependOn("A", "B")
			g.DependOn("B", "C")
			g.DependOn("C", "D")
			g.DependOn("D", "E")
			Expect(g.TopoSortedLayers()).To(Equal([][]string{{"E"}, {"D"}, {"C"}, {"B"}, {"A"}}))
		})

		It("orders parallel", func() {
			g.DependOn("A", "B")
			g.DependOn("B", "C")
			g.DependOn("C", "D")
			g.DependOn("D", "E")
			g.DependOn("X", "E")
			Expect(g.TopoSortedLayers()).To(
				Or(
					Equal([][]string{{"E"}, {"D", "X"}, {"C"}, {"B"}, {"A"}}),
					Equal([][]string{{"E"}, {"X", "D"}, {"C"}, {"B"}, {"A"}}),
				),
			)
		})
	})

	Context("Sequential runs", func() {
		It("orders parallel", func() {
			f := ""
			g.Add("foo", WithCallback(func(ctx context.Context) error {
				f += "foo"
				return nil
			}), WithDeps("bar"))
			g.Add("bar", WithCallback(func(ctx context.Context) error {
				f += "bar"
				return nil
			}))
			g.Run(context.Background())
			Expect(f).To(Equal("barfoo"))
		})
	})

	Context("With errors", func() {
		It("fails", func() {
			f := ""

			g.Add("foo", WithCallback(func(ctx context.Context) error {
				return fmt.Errorf("failure")
			}), WithDeps("bar"), FatalOp)

			g.Add("bar",
				WithCallback(func(ctx context.Context) error {
					f += "bar"
					return nil
				}),
			)

			err := g.Run(context.Background())
			Expect(err.Error()).To(ContainSubstring("failure"))
		})
	})

	Context("Sequential runs, background jobs", func() {
		It("orders parallel", func() {
			testChan := make(chan string)
			f := ""
			g.Add("foo", WithCallback(func(ctx context.Context) error {
				f += "triggered"
				return nil
			}), WithDeps("bar"))
			g.Add("bar", WithCallback(func(ctx context.Context) error {
				<-testChan
				return fmt.Errorf("test")
			}), Background)
			g.Run(context.Background())
			Expect(g.State("bar").Error).ToNot(HaveOccurred())
			Expect(f).To(Equal("triggered"))
			testChan <- "foo"
			Eventually(func() error {
				return g.State("bar").Error
			}).Should(HaveOccurred())
		})
	})

	Context("Weak deps", func() {
		It("runs with weak deps", func() {
			f := ""
			g.Add("foo", WithCallback(func(ctx context.Context) error {
				f += "triggered"
				return nil
			}), WithDeps("bar"), WeakDeps)
			g.Add("bar", WithCallback(func(ctx context.Context) error {
				return fmt.Errorf("test")
			}))

			g.Run(context.Background())
			Expect(f).To(Equal("triggered"))
		})
		It("doesn't run without weak deps", func() {
			f := ""
			foo := ""
			g.Add("foo", WithCallback(func(ctx context.Context) error {
				foo = "triggered"
				return nil
			}), WithDeps("bar"))

			g.Add("fooz", WithCallback(func(ctx context.Context) error {
				f = "nomercy"
				return nil
			}), WithDeps("baz"))

			g.Add("baz", WithCallback(func(ctx context.Context) error {
				return nil
			}))

			g.Add("bar", WithCallback(func(ctx context.Context) error {
				return fmt.Errorf("test")
			}))

			err := g.Run(context.Background())
			Expect(err).ToNot(HaveOccurred())

			Expect(g.State("bar").Error).To(HaveOccurred())
			Expect(f).To(Equal("nomercy"))
			Expect(foo).To(Equal(""))
		})
	})

	Context("init", func() {
		var baz bool
		var foo bool

		BeforeEach(func() {
			baz = false
			foo = false
		})

		It("does not run untied jobs", func() {
			g.Add("baz", WithCallback(func(ctx context.Context) error {
				baz = true
				return nil
			}))

			g.Add("foo", WithCallback(func(ctx context.Context) error {
				foo = true
				return nil
			}))

			err := g.Run(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(foo).To(BeFalse())
			Expect(baz).To(BeFalse())
		})

		It("does run all untied jobs", func() {
			g = DAG(EnableInit)
			Expect(g).ToNot(BeNil())

			g.Add("baz", WithCallback(func(ctx context.Context) error {
				baz = true
				return nil
			}))

			g.Add("foo", WithCallback(func(ctx context.Context) error {
				foo = true
				return nil
			}))

			err := g.Run(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(foo).To(BeTrue())
			Expect(baz).To(BeTrue())
		})
	})

	Context("Background jobs", func() {
		It("waits for background jobs to finish", func() {
			g = DAG(CollectOrphans)
			Expect(g).ToNot(BeNil())

			g.Add("baz",
				Background,
				FatalOp,
				WithCallback(func(ctx context.Context) error {
					return fmt.Errorf("failure")
				}))

			g.Add("foo",
				WithDeps("baz"),
				WithCallback(func(ctx context.Context) error {
					return nil
				}))

			err := g.Run(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failure"))
		})
	})

	Context("Multiple callbacks", func() {
		It("fails if one of them fail", func() {
			f := ""

			g.Add("foo", WithCallback(func(ctx context.Context) error {
				return nil
			}, func(ctx context.Context) error {
				return fmt.Errorf("ohno")
			}), WithDeps("bar"), FatalOp)

			g.Add("bar",
				WithCallback(func(ctx context.Context) error {
					f += "bar"
					return nil
				}),
			)

			err := g.Run(context.Background())
			Expect(err.Error()).To(ContainSubstring("ohno"))
		})

		It("runs sequentially", func() {
			f := ""
			mu := sync.Mutex{}
			g.Add("foo", WithCallback(
				// Those runs in parallel
				func(ctx context.Context) error {
					mu.Lock()
					f += "foo"
					mu.Unlock()
					return nil
				},
				func(ctx context.Context) error {
					mu.Lock()
					f += "na"
					mu.Unlock()
					return nil
				},
			), WithDeps("bar"))
			g.Add("bar", WithCallback(func(ctx context.Context) error {
				f += "bar"
				return nil
			}))
			g.Run(context.Background())
			Expect(f).To(Or(Equal("barfoona"), Equal("barnafoo")))
		})

		It("ignores ops", func() {
			g = DAG(EnableInit)
			Expect(g).ToNot(BeNil())
			f := ""
			mu := sync.Mutex{}
			g.Add("foo", WithCallback(
				// Those runs in parallel
				func(ctx context.Context) error {
					mu.Lock()
					f += "foo"
					mu.Unlock()
					return nil
				},
				func(ctx context.Context) error {
					mu.Lock()
					f += "na"
					mu.Unlock()
					return nil
				},
			))
			g.Run(context.Background())
			Expect(f).To(Or(Equal("foona"), Equal("nafoo")))
		})

		Context("specific weak dep", func() {
			It("runs with weak deps", func() {
				f := ""
				g.Add("foo", WithCallback(func(ctx context.Context) error {
					f += "triggered"
					return nil
				}), WithDeps("baz"), WithWeakDeps("bar"))
				g.Add("bar", WithCallback(func(ctx context.Context) error {
					return fmt.Errorf("test")
				}))
				g.Add("baz", WithCallback(func(ctx context.Context) error {
					return nil
				}))
				g.Run(context.Background())
				Expect(f).To(Equal("triggered"))
			})

			It("doesn't run with weak deps if a hard dep fail", func() {
				f := ""
				g.Add("foo", WithCallback(func(ctx context.Context) error {
					f += "triggered"
					return nil
				}), WithDeps("bar"), WithWeakDeps("baz"))
				g.Add("bar", WithCallback(func(ctx context.Context) error {
					return fmt.Errorf("test")
				}))
				g.Add("baz", WithCallback(func(ctx context.Context) error {
					return nil
				}))
				g.Run(context.Background())
				Expect(f).To(BeEmpty())
			})

			It("runs with weak deps also if specifying twice", func() {
				f := ""
				g.Add("foo", WithCallback(func(ctx context.Context) error {
					f += "triggered"
					return nil
				}), WithDeps("baz", "bar"), WithWeakDeps("bar"))
				g.Add("bar", WithCallback(func(ctx context.Context) error {
					return fmt.Errorf("test")
				}))
				g.Add("baz", WithCallback(func(ctx context.Context) error {
					return nil
				}))
				Expect(g.Run(context.Background())).ToNot(HaveOccurred())
				Expect(f).To(Equal("triggered"))

				Expect(len(g.Analyze())).To(Equal(2))
				Expect(len(g.Analyze()[0])).To(Equal(2))
				Expect(len(g.Analyze()[1])).To(Equal(1))

				for _, layer := range g.Analyze() {
					for _, f := range layer {
						Expect(f.Executed).To(BeTrue(), fmt.Sprintf("%+v", f))
					}
				}
			})
		})
	})
})
