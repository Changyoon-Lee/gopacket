package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pfring"
)

type FlowKey struct {
	SrcIP   string
	SrcPort string
	DstIP   string
	DstPort string
}

type FlowStats struct {
	Bytes int
	Pkts  int
}

func main() {
	device := "en0" // 네트워크 인터페이스 선택 (macOS에서는 en0일 수도 있음)

	// 패킷 캡처를 위한 핸들 생성
	handle, err := pfring.NewRing(device, 1500, pfring.FlagPromisc)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// 집계를 저장할 맵 초기화
	flowMap := make(map[FlowKey]FlowStats)

	// 패킷 캡처 시작
	packetSource := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)

	// 패킷 처리를 위한 고루틴 개수
	numWorkers := 4

	// 각 고루틴의 결과를 저장할 채널 생성
	resultChan := make(chan map[FlowKey]FlowStats, numWorkers)

	// 각 고루틴 시작
	for i := 0; i < numWorkers; i++ {
		go packetWorker(packetSource.Packets(), resultChan)
	}

	// 10초 타이머 생성
	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case result := <-resultChan:
			// 고루틴 결과를 집계 맵에 병합
			for key, stats := range result {
				flowMap[key] = FlowStats{
					Bytes: flowMap[key].Bytes + stats.Bytes,
					Pkts:  flowMap[key].Pkts + stats.Pkts,
				}
			}

		case <-ticker.C:
			// 10초마다 집계 결과 출력
			fmt.Println("Flow Stats (10 Seconds):", time.Now())
			for key, stats := range flowMap {
				fmt.Printf("SrcIP: %s, SrcPort: %s, DstIP: %s, DstPort: %s, SrcBytes: %d, SrcPkts: %d\n",
					key.SrcIP, key.SrcPort, key.DstIP, key.DstPort, stats.Bytes, stats.Pkts)
				// dst_bytes, dst_pkts도 위와 같은 방식으로 출력
			}
			fmt.Println()

			// 집계 맵 초기화
			flowMap = make(map[FlowKey]FlowStats)
		}
	}
}

func packetWorker(packetChan <-chan gopacket.Packet, resultChan chan<- map[FlowKey]FlowStats) {
	// 집계 결과를 저장할 맵 초기화
	flowMap := make(map[FlowKey]FlowStats)

	for packet := range packetChan {
		// 패킷 처리
		// 필요한 필드 추출
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			srcIP := ip.SrcIP.String()
			dstIP := ip.DstIP.String()

			tcpLayer := packet.Layer(layers.LayerTypeTCP)
			if tcpLayer != nil {
				tcp, _ := tcpLayer.(*layers.TCP)
				srcPort := tcp.SrcPort.String()
				dstPort := tcp.DstPort.String()

				// 집계 키 생성
				key := FlowKey{SrcIP: srcIP, SrcPort: srcPort, DstIP: dstIP, DstPort: dstPort}

				// 집계 업데이트
				stats := flowMap[key]
				stats.Bytes += len(tcp.Payload)
				stats.Pkts++
				flowMap[key] = stats
			}
		}
	}

	// 고루틴이 종료되면 결과를 채널로 보내기
	resultChan <- flowMap
}
