package main

import (
	"io"
	"log"
	"net"
	"testing"

	g "github.com/dictav/test-go-grpc-flatbuffers/grpcexample"
	fb "github.com/google/flatbuffers/go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var client g.GRPCExampleClient

type serviceHandler struct{}

func buildPerson(n int32) *fb.Builder {
	b := fb.NewBuilder(0)
	b.Finish(_buildPerson(b, n))

	return b
}

func _buildPerson(b *fb.Builder, n int32) fb.UOffsetT {
	ns := b.CreateString("name")
	es := b.CreateString("email")

	plen := int(n%3 + 1)
	phones := make([]fb.UOffsetT, plen)
	for i := 0; i < plen; i++ {
		pn := b.CreateString("phone")
		g.PhoneNumberStart(b)
		g.PhoneNumberAddNumber(b, pn)
		g.PhoneNumberAddPtype(b, int8(n))
		phones[i] = g.PhoneNumberEnd(b)
	}

	g.PersonStartPhoneVector(b, plen)
	for i := 0; i < plen; i++ {
		b.PrependUOffsetT(phones[i])
	}
	ps := b.EndVector(plen)

	g.PersonStart(b)
	g.PersonAddId(b, n)
	g.PersonAddName(b, ns)
	g.PersonAddEmail(b, es)
	g.PersonAddPhone(b, ps)
	return g.PersonEnd(b)
}

func (h *serviceHandler) GetPerson(ctx context.Context, in *g.Request) (*fb.Builder, error) {
	return buildPerson(0), nil
}

func (h *serviceHandler) ListPeople(in *g.Request, stream g.GRPCExample_ListPeopleServer) error {
	for i := int32(0); i < 100; i++ {
		stream.Send(buildPerson(i))
	}

	return nil
}

func (h *serviceHandler) ArrayPeople(ctx context.Context, in *g.Request) (*fb.Builder, error) {
	b := fb.NewBuilder(0)
	people := make([]fb.UOffsetT, 100)
	for i := int32(0); i < 100; i++ {
		people[i] = _buildPerson(b, i)
	}

	g.ResultStartItemsVector(b, 100)
	for i := 0; i < 100; i++ {
		b.PrependUOffsetT(people[i])
	}
	items := b.EndVector(100)

	g.ResultStart(b)
	g.ResultAddItems(b, items)
	b.Finish(g.ResultEnd(b))
	return b, nil
}

func serve() {
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatal(err)
	}
	s := grpc.NewServer(grpc.CustomCodec(fb.FlatbuffersCodec{}))
	h := &serviceHandler{}
	g.RegisterGRPCExampleServer(s, h)
	log.Fatal(s.Serve(lis))
}

func TestMain(m *testing.M) {
	go serve()
	conn, err := grpc.Dial(":9090", grpc.WithInsecure(), grpc.WithCodec(fb.FlatbuffersCodec{}))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client = g.NewGRPCExampleClient(conn)

	m.Run()
}

func buildRequest() *fb.Builder {
	b := fb.NewBuilder(0)

	g.RequestStart(b)
	b.Finish(g.RequestEnd(b))

	return b
}

func BenchmarkGetPerson(b *testing.B) {
	ctx := context.Background()
	req := buildRequest()

	for i := 0; i < b.N; i++ {
		if _, err := client.GetPerson(ctx, req); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkListPerson(b *testing.B) {
	ctx := context.Background()
	req := buildRequest()
	names := make([][]byte, b.N)

	for i := 0; i < b.N; i++ {
		strm, err := client.ListPeople(ctx, req)
		if err != nil {
			b.Error(err)
		}

		for {
			res, err := strm.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Error(err)
			}
			names[i] = res.Name()
		}
	}
}

func BenchmarkArrayPerson(b *testing.B) {
	ctx := context.Background()
	req := buildRequest()

	for i := 0; i < b.N; i++ {
		_, err := client.ArrayPeople(ctx, req)
		if err != nil {
			b.Error(err)
		}
	}
}
