// beef-gen emits BRC-148 BEEF submission records to a proxy ingress port.
//
// A submission record is the open-class envelope the tx port (8725) and the
// optional dedicated BEEF lane (8728) both accept:
//
//	u16 tag 0xBEEF ∥ u8 recordVer ∥ u8 topicCount ∥ topics ∥ u32 objectLen ∥ object
//
// One record names one BEEF object to one or more topics; the proxy expands
// it into one FrameVer 0x09 multicast frame per topic. Objects are either
// synthetic (a valid BEEF-family leading marker followed by seeded bytes —
// the fabric never parses past the marker) or the real BRC-62 specification
// example (-encoding real), which proves verbatim carriage end to end.
//
// Every synthetic object embeds the emission counter, so ContentIDs are
// unique per emission and the proxy's (ContentID, TopicID) ingress dedup
// never suppresses generator traffic. Every record is self-verified with
// objfmt.BEEFRecordSize before writing, so a malformed emitter can never
// masquerade as a fabric/proxy fault.
package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lightwebinc/shard-common/objfmt"
)

// beefVectorHex is the worked example from the BRC-62 specification: a real
// two-transaction BEEF with one BUMP. Sent verbatim by -encoding real.
const beefVectorHex = "0100beef01fe636d0c0007021400fe507c0c7aa754cef1f7889d5fd395cf1f785dd7de98eed895dbedfe4e5bc70d1502ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e010b00bc4ff395efd11719b277694cface5aa50d085a0bb81f613f70313acd28cf4557010400574b2d9142b8d28b61d88e3b2c3f44d858411356b49a28a4643b6d1a6a092a5201030051a05fc84d531b5d250c23f4f886f6812f9fe3f402d61607f977b4ecd2701c19010000fd781529d58fc2523cf396a7f25440b409857e7e221766c57214b1d38c7b481f01010062f542f45ea3660f86c013ced80534cb5fd4c19d66c56e7e8c5d4bf2d40acc5e010100b121e91836fd7cd5102b654e9f72f3cf6fdbfd0b161c53a9c54b12c841126331020100000001cd4e4cac3c7b56920d1e7655e7e260d31f29d9a388d04910f1bbd72304a79029010000006b483045022100e75279a205a547c445719420aa3138bf14743e3f42618e5f86a19bde14bb95f7022064777d34776b05d816daf1699493fcdf2ef5a5ab1ad710d9c97bfb5b8f7cef3641210263e2dee22b1ddc5e11f6fab8bcd2378bdd19580d640501ea956ec0e786f93e76ffffffff013e660000000000001976a9146bfd5c7fbe21529d45803dbcf0c87dd3c71efbc288ac0000000001000100000001ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e000000006a47304402203a61a2e931612b4bda08d541cfb980885173b8dcf64a3471238ae7abcd368d6402204cbf24f04b9aa2256d8901f0ed97866603d2be8324c2bfb7a37bf8fc90edd5b441210263e2dee22b1ddc5e11f6fab8bcd2378bdd19580d640501ea956ec0e786f93e76ffffffff013c660000000000001976a9146bfd5c7fbe21529d45803dbcf0c87dd3c71efbc288ac0000000000"

func main() {
	var (
		addr        = flag.String("addr", "[::1]:8725", "proxy ingress TCP address (open tx port, or the dedicated BEEF lane)")
		topicsFlag  = flag.String("topics", "tm_demo", "comma-separated overlay topic names for every submission")
		encoding    = flag.String("encoding", "beef", "object encoding: beef|beefv2|atomic|real (real = BRC-62 spec example, verbatim)")
		objectBytes = flag.Int("object-bytes", 64, "synthetic object size in bytes (>= 16; ignored by -encoding real)")
		count       = flag.Int("count", 10, "number of submissions (0 = unlimited; bounded by -duration)")
		interval    = flag.Duration("interval", 50*time.Millisecond, "delay between submissions")
		duration    = flag.Duration("duration", 0, "stop after this long (0 = count-driven)")
		seed        = flag.Int64("seed", 1, "deterministic synthetic-object seed (also identifies this source)")
		logHashes   = flag.Bool("log-hashes", false, "log each emission's ContentID")
	)
	flag.Parse()

	topics := splitTopics(*topicsFlag)
	if len(topics) == 0 {
		log.Fatal("beef-gen: -topics must name at least one topic")
	}
	if *objectBytes < 16 {
		log.Fatal("beef-gen: -object-bytes must be >= 16")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	rng := rand.New(rand.NewSource(*seed))
	conn := dial(ctx, *addr)
	if conn == nil {
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()

	sent := 0
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for *count == 0 || sent < *count {
		obj, err := buildObject(*encoding, *objectBytes, uint64(sent), rng)
		if err != nil {
			log.Fatalf("beef-gen: %v", err)
		}
		rec, err := objfmt.EncodeBEEFRecord(topics, obj)
		if err != nil {
			log.Fatalf("beef-gen: encode record: %v", err)
		}
		// Self-verify before every write: a malformed emitter must never
		// masquerade as a fabric fault.
		if n, err := objfmt.BEEFRecordSize(rec); err != nil || n != len(rec) {
			log.Fatalf("beef-gen: record self-verify failed: n=%d err=%v", n, err)
		}

		if _, err := conn.Write(rec); err != nil {
			log.Printf("beef-gen: write error (%v); reconnecting", err)
			_ = conn.Close()
			conn = dial(ctx, *addr)
			if conn == nil {
				break
			}
			continue // retry this emission on the fresh connection
		}
		if *logHashes {
			cid := objfmt.ContentID(obj)
			log.Printf("beef-gen: sent topics=%v content_id=%x bytes=%d", topics, cid[:8], len(obj))
		}
		sent++

		select {
		case <-ctx.Done():
			fmt.Printf("sent=%d\n", sent)
			return
		case <-ticker.C:
		}
	}
	fmt.Printf("sent=%d\n", sent)
}

// splitTopics parses the comma-separated topic list.
func splitTopics(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// buildObject assembles one BEEF object for emission n.
func buildObject(encoding string, size int, n uint64, rng *rand.Rand) ([]byte, error) {
	var marker []byte
	switch encoding {
	case "beef":
		marker = []byte{0x01, 0x00, 0xBE, 0xEF}
	case "beefv2":
		marker = []byte{0x02, 0x00, 0xBE, 0xEF}
	case "atomic":
		marker = []byte{0x01, 0x01, 0x01, 0x01}
	case "real":
		obj, err := hex.DecodeString(beefVectorHex)
		if err != nil {
			return nil, fmt.Errorf("decode BRC-62 vector: %w", err)
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown -encoding %q (beef|beefv2|atomic|real)", encoding)
	}

	obj := make([]byte, size)
	copy(obj, marker)
	rng.Read(obj[len(marker) : size-8])
	// The trailing emission counter keeps every synthetic ContentID unique,
	// so ingress (ContentID, TopicID) dedup never suppresses the generator.
	binary.BigEndian.PutUint64(obj[size-8:], n)
	if !objfmt.IsBEEFObject(obj) {
		return nil, fmt.Errorf("synthetic object failed marker self-check")
	}
	return obj, nil
}

// dial connects with exponential backoff until ctx is done.
func dial(ctx context.Context, addr string) net.Conn {
	backoff := 100 * time.Millisecond
	for {
		d := net.Dialer{Timeout: 2 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			return conn
		}
		log.Printf("beef-gen: dial %s: %v (retry in %v)", addr, err, backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}
