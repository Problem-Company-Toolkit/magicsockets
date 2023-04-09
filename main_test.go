package magicsockets_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/problem-company-toolkit/magicsockets"
)

var _ = Describe("Main", func() {
	var (
		ms      magicsockets.MagicSocket
		address string

		key    string
		topics []string
	)

	BeforeEach(func() {
		// Very unlikely to be allocated or to conflict in parallel tests.
		randomPort := gofakeit.IntRange(10000, 30000)
		address = fmt.Sprintf("0.0.0.0:%d", randomPort)
		ms = magicsockets.New(magicsockets.MagicSocketOpts{
			Port: randomPort,
		})

		key = gofakeit.UUID()
		topics = []string{gofakeit.BuzzWord(), gofakeit.Adjective(), gofakeit.PetName()}

		go ms.Start()
		time.Sleep(time.Millisecond * 100)
	})

	AfterEach(func() {
		if err := ms.Stop(); err != nil {
			panic(err)
		}
		ms = nil
	})

	It("Receives a websocket connection", func() {
		client, err := newTestClient(address)
		Expect(err).ToNot(HaveOccurred())

		err = client.Close()
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Updates a client key", func() {
		ms.SetOnConnect(func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
			return magicsockets.RegisterClientOpts{
				Key: key,
			}, nil
		})

		client, err := newTestClient(address)
		Expect(err).ToNot(HaveOccurred())

		clients := ms.GetClients()
		connectionClient, ok := clients[key]

		Expect(ok).To(BeTrue())
		Expect(connectionClient).ToNot(BeNil())
		Expect(connectionClient.GetKey()).To(BeEquivalentTo(key))

		newKey := gofakeit.UUID()
		err = ms.UpdateClientKey(key, newKey)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(connectionClient).ToNot(BeNil())

		Expect(connectionClient.GetKey()).ToNot(BeEquivalentTo(newKey))

		clients = ms.GetClients()
		connectionClient, ok = clients[newKey]

		Expect(ok).To(BeTrue())
		Expect(connectionClient).ToNot(BeNil())
		Expect(connectionClient.GetKey()).To(BeEquivalentTo(newKey))

		_, ok = clients[key]
		Expect(ok).To(BeFalse())

		err = client.Close()
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Registers a client connection with a specific key and topic", func() {
		ms.SetOnConnect(func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
			return magicsockets.RegisterClientOpts{
				Key:    key,
				Topics: topics,
			}, nil
		})

		client, err := newTestClient(address)
		Expect(err).ToNot(HaveOccurred())

		err = client.Close()
		Expect(err).ShouldNot(HaveOccurred())

		clients := ms.GetClients()
		Expect(clients[key]).ToNot(BeNil())
		Expect(clients[key].GetKey()).To(BeEquivalentTo(key))
		Expect(clients[key].GetTopics()).To(BeEquivalentTo(topics))
	})

	It("Updates a client's topics", func() {
		ms.SetOnConnect(func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
			return magicsockets.RegisterClientOpts{
				Key:    key,
				Topics: topics,
			}, nil
		})

		client, err := newTestClient(address)
		Expect(err).ToNot(HaveOccurred())

		err = client.Close()
		Expect(err).ShouldNot(HaveOccurred())

		clients := ms.GetClients()
		Expect(clients[key]).ToNot(BeNil())
		Expect(clients[key].GetKey()).To(BeEquivalentTo(key))
		Expect(clients[key].GetTopics()).To(BeEquivalentTo(topics))

		newTopics := []string{gofakeit.BuzzWord(), gofakeit.BuzzWord(), gofakeit.BuzzWord(), gofakeit.BuzzWord()}
		clients[key].UpdateTopics(newTopics)
		Expect(clients[key].GetTopics()).To(BeEquivalentTo(newTopics))
	})

	It("Triggers OnOutgoing correctly", func() {
		outgoingTriggered := make(chan bool)

		testMessage := "my test message 123"
		ms.SetOnConnect(func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
			return magicsockets.RegisterClientOpts{
				Key:    key,
				Topics: topics,
				OnOutgoing: func(messageType int, data []byte) error {
					outgoingTriggered <- true
					return nil
				},
			}, nil
		})

		client, err := newTestClient(address)
		Expect(err).ToNot(HaveOccurred())

		ms.Emit(magicsockets.EmitOpts{
			Rules: []magicsockets.EmitRule{
				{
					Keys: []string{key},
				},
			},
		}, []byte(testMessage))

		Eventually(<-outgoingTriggered).Should(BeTrue())

		client.Close()
	})

	It("Triggers OnIncoming correctly", func() {
		incomingTriggered := make(chan bool)

		ms.SetOnConnect(func(r *http.Request) (magicsockets.RegisterClientOpts, error) {
			return magicsockets.RegisterClientOpts{
				Key:    key,
				Topics: topics,
				OnIncoming: func(messageType int, data []byte) error {
					incomingTriggered <- true
					return nil
				},
			}, nil
		})

		client, err := newTestClient(address)
		Expect(err).ToNot(HaveOccurred())

		client.WriteMessage(websocket.TextMessage, []byte("test message"))
		Eventually(<-incomingTriggered).Should(BeTrue())

		client.Close()
	})
})
