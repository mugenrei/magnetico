package mainline

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type IndexingService struct {
	// Private
	protocol      *Protocol
	started       bool
	interval      time.Duration
	eventHandlers IndexingServiceEventHandlers

	nodeID []byte
	// []byte type would be a much better fit for the keys but unfortunately (and quite
	// understandably) slices cannot be used as keys (since they are not hashable), and using arrays
	// (or even the conversion between each other) is a pain; hence map[string]net.UDPAddr
	//                                                                  ^~~~~~
	routingTable      map[string]*net.UDPAddr
	routingTableMutex sync.RWMutex
	maxNeighbors      uint

	counter          uint16
	getPeersRequests map[[2]byte][20]byte // GetPeersQuery.`t` -> infohash
}

type IndexingServiceEventHandlers struct {
	OnResult func(IndexingResult)
}

type IndexingResult struct {
	infoHash  [20]byte
	peerAddrs []net.TCPAddr
}

func (ir IndexingResult) InfoHash() [20]byte {
	return ir.infoHash
}

func (ir IndexingResult) PeerAddrs() []net.TCPAddr {
	return ir.peerAddrs
}

func NewIndexingService(laddr string, interval time.Duration, maxNeighbors uint, eventHandlers IndexingServiceEventHandlers) *IndexingService {
	service := new(IndexingService)
	service.interval = interval
	service.protocol = NewProtocol(
		laddr,
		ProtocolEventHandlers{
			OnFindNodeResponse:         service.onFindNodeResponse,
			OnGetPeersResponse:         service.onGetPeersResponse,
			OnSampleInfohashesResponse: service.onSampleInfohashesResponse,
		},
	)
	service.nodeID = make([]byte, 20)
	service.routingTable = make(map[string]*net.UDPAddr)
	service.maxNeighbors = maxNeighbors
	service.eventHandlers = eventHandlers

	service.getPeersRequests = make(map[[2]byte][20]byte)

	return service
}

func (is *IndexingService) Start() {
	if is.started {
		zap.L().Panic("Attempting to Start() a mainline/IndexingService that has been already started! (Programmer error.)")
	}
	is.started = true

	is.protocol.Start()
	go is.index()

	zap.L().Info("Indexing Service started!")
}

func (is *IndexingService) Terminate() {
	is.protocol.Terminate()
}

func (is *IndexingService) index() {
	for range time.Tick(is.interval) {
		is.routingTableMutex.RLock()
		routingTableLen := len(is.routingTable)
		is.routingTableMutex.RUnlock()
		if routingTableLen == 0 {
			is.bootstrap()
		} else {
			zap.L().Debug("Latest status:", zap.Int("n", routingTableLen),
				zap.Uint("maxNeighbors", is.maxNeighbors))
			//TODO
			is.findNeighbors()
			is.routingTableMutex.Lock()
			is.routingTable = make(map[string]*net.UDPAddr)
			is.routingTableMutex.Unlock()
		}
	}
}

func (is *IndexingService) bootstrap() {
	bootstrappingNodes := []string{
		"tracker.opentrackr.org:1337",
		"opentracker.i2p.rocks:6969",
		"tracker.openbittorrent.com:6969",
		"open.demonii.com:1337",
		"open.stealth.si:80",
		"exodus.desync.com:6969",
		"tracker1.bt.moack.co.kr:80",
		"tracker.moeking.me:6969",
		"movies.zsw.ca:6969",
		"uploads.gamecoast.net:6969",
		"tracker.theoks.net:6969",
		"tracker.joybomb.tw:6969",
		"tracker.filemail.com:6969",
		"tracker.auctor.tv:6969",
		"tracker.4.babico.name.tr:3131",
		"sanincode.com:6969",
		"retracker01-msk-virt.corbina.net:80",
		"private.anonseed.com:6969",
		"p4p.arenabg.com:1337",
		"htz3.noho.st:6969",
		"epider.me:6969",
		"bt.ktrackers.com:6666",
		"acxx.de:6969",
		"aarsen.me:6969",
		"v1046920.hosted-by-vdsina.ru:6969",
		"tracker2.dler.org:80",
		"tracker.tiny-vps.com:6969",
		"tracker.leech.ie:1337",
		"tracker.bittor.pw:1337",
		"tk1.trackerservers.com:8080",
		"thouvenin.cloud:6969",
		"tamas3.ynh.fr:6969",
		"opentracker.io:6969",
		"open.free-tracker.ga:6969",
		"open.dstud.io:6969",
		"new-line.net:6969",
		"moonburrow.club:6969",
		"inferno.demonoid.is:3391",
		"download.nerocloud.me:6969",
		"carr.codes:6969",
		"bt2.archive.org:6969",
		"bt1.archive.org:6969",
		"black-bird.ynh.fr:6969",
		"6ahddutb1ucc3cp.ru:6969",
		"wepzone.net:6969",
		"tracker1.myporn.club:9337",
		"tracker.cubonegro.lol:6969",
		"tracker.ccp.ovh:6969",
		"thinking.duckdns.org:6969",
		"t.zerg.pw:6969",
		"ryjer.com:6969",
		"run-2.publictracker.xyz:6969",
		"public.tracker.vraphim.com:6969",
		"6.pocketnet.app:6969",
		"yahor.of.by:6969",
		"tracker.qu.ax:6969",
		"tracker.ocnix.net:6969",
		"tracker.farted.net:6969",
		"tracker.dler.org:6969",
		"tracker.army:6969",
		"tracker.0x7c0.com:6969",
		"su-data.com:6969",
		"ssb14.nohost.me:6969",
		"public-tracker.cf:6969",
		"open.u-p.pw:6969",
		"oh.fuuuuuck.com:6969",
		"ns1.monolithindustries.com:6969",
		"market-re.quest:6969",
		"mail.segso.net:6969",
		"freedomalternative.com:6969",
		"free.publictracker.xyz:6969",
		"1c.premierzal.ru:6969",
		"tracker2.itzmx.com:6961",
		"tracker.srv00.com:6969",
		"tracker.ddunlimited.net:6969",
		"tracker.artixlinux.org:6969",
		"tracker-udp.gbitt.info:80",
		"tr.bangumi.moe:6969",
		"torrents.artixlinux.org:6969",
		"psyco.fr:6969",
		"mail.artixlinux.org:6969",
		"fh2.cmp-gaming.com:6969",
		"concen.org:6969",
		"boysbitte.be:6969",
		"aegir.sexy:6969",
	}

	zap.L().Info("Bootstrapping as routing table is empty...")
	for _, node := range bootstrappingNodes {
		target := make([]byte, 20)
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes during bootstrapping!")
		}

		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			zap.L().Error("Could NOT resolve (UDP) address of the bootstrapping node!",
				zap.String("node", node))
			continue
		}

		is.protocol.SendMessage(NewFindNodeQuery(is.nodeID, target), addr)
	}
}

func (is *IndexingService) findNeighbors() {
	target := make([]byte, 20)

	/*
		We could just RLock and defer RUnlock here, but that would mean that each response that we get could not Lock
		the table because we are sending. So we would basically make read and write NOT concurrent.
		A better approach would be to get all addresses to send in a slice and then work on that, releasing the main map.
	*/
	is.routingTableMutex.RLock()
	addressesToSend := make([]*net.UDPAddr, 0, len(is.routingTable))
	for _, addr := range is.routingTable {
		addressesToSend = append(addressesToSend, addr)
	}
	is.routingTableMutex.RUnlock()

	for _, addr := range addressesToSend {
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes during bootstrapping!")
		}

		is.protocol.SendMessage(
			NewSampleInfohashesQuery(is.nodeID, []byte("aa"), target),
			addr,
		)
	}
}

func (is *IndexingService) onFindNodeResponse(response *Message, addr *net.UDPAddr) {
	is.routingTableMutex.Lock()
	defer is.routingTableMutex.Unlock()

	for _, node := range response.R.Nodes {
		if uint(len(is.routingTable)) >= is.maxNeighbors {
			break
		}
		if node.Addr.Port == 0 { // Ignore nodes who "use" port 0.
			continue
		}

		is.routingTable[string(node.ID)] = &node.Addr

		target := make([]byte, 20)
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes!")
		}
		is.protocol.SendMessage(
			NewSampleInfohashesQuery(is.nodeID, []byte("aa"), target),
			&node.Addr,
		)
	}
}

func (is *IndexingService) onGetPeersResponse(msg *Message, addr *net.UDPAddr) {
	var t [2]byte
	copy(t[:], msg.T)

	infoHash := is.getPeersRequests[t]
	// We got a response, so free the key!
	delete(is.getPeersRequests, t)

	// BEP 51 specifies that
	//     The new sample_infohashes remote procedure call requests that a remote node return a string of multiple
	//     concatenated infohashes (20 bytes each) FOR WHICH IT HOLDS GET_PEERS VALUES.
	//                                                                          ^^^^^^
	// So theoretically we should never hit the case where `values` is empty, but c'est la vie.
	if len(msg.R.Values) == 0 {
		return
	}

	peerAddrs := make([]net.TCPAddr, 0)
	for _, peer := range msg.R.Values {
		if peer.Port == 0 {
			continue
		}

		peerAddrs = append(peerAddrs, net.TCPAddr{
			IP:   peer.IP,
			Port: peer.Port,
		})
	}

	is.eventHandlers.OnResult(IndexingResult{
		infoHash:  infoHash,
		peerAddrs: peerAddrs,
	})
}

func (is *IndexingService) onSampleInfohashesResponse(msg *Message, addr *net.UDPAddr) {
	// request samples
	for i := 0; i < len(msg.R.Samples)/20; i++ {
		var infoHash [20]byte
		copy(infoHash[:], msg.R.Samples[i:(i+1)*20])

		msg := NewGetPeersQuery(is.nodeID, infoHash[:])
		t := uint16BE(is.counter)
		msg.T = t[:]

		is.protocol.SendMessage(msg, addr)

		is.getPeersRequests[t] = infoHash
		is.counter++
	}

	// TODO: good idea, but also need to track how long they have been here
	//if msg.R.Num > len(msg.R.Samples) / 20 &&  time.Duration(msg.R.Interval) <= is.interval {
	//	if addr.Port != 0 {  // ignore nodes who "use" port 0...
	//		is.routingTable[string(msg.R.ID)] = addr
	//	}
	//}

	// iterate
	is.routingTableMutex.Lock()
	defer is.routingTableMutex.Unlock()
	for _, node := range msg.R.Nodes {
		if uint(len(is.routingTable)) >= is.maxNeighbors {
			break
		}
		if node.Addr.Port == 0 { // Ignore nodes who "use" port 0.
			continue
		}
		is.routingTable[string(node.ID)] = &node.Addr

		// TODO
		/*
			target := make([]byte, 20)
			_, err := rand.Read(target)
			if err != nil {
				zap.L().Panic("Could NOT generate random bytes!")
			}
			is.protocol.SendMessage(
				NewSampleInfohashesQuery(is.nodeID, []byte("aa"), target),
				&node.Addr,
			)
		*/
	}
}

func uint16BE(v uint16) (b [2]byte) {
	b[0] = byte(v >> 8)
	b[1] = byte(v)
	return
}
