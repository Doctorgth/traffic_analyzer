package main

import (
	"context"
	"encoding/binary"
	"net"
	"sync"

	"github.com/armon/go-socks5"
)

var (
	InjectQueue = make(chan []byte, 1024)
)

func StartProxy(cfg *Config, ui *AppUI) {
	creds := socks5.StaticCredentials{cfg.User: cfg.Pass}
	conf := &socks5.Config{
		AuthMethods: []socks5.Authenticator{socks5.UserPassAuthenticator{Credentials: creds}},
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			_, port, _ := net.SplitHostPort(addr)
			if port != cfg.GamePort {
				return dest, nil
			}
			return WrapConn(dest, ui), nil
		},
	}
	srv, _ := socks5.New(conf)
	l, _ := net.Listen("tcp", "0.0.0.0:"+cfg.ProxyPort)
	srv.Serve(l)
}

type Interceptor struct {
	net.Conn
	ui            *AppUI
	streamBuf     []byte // Буфер накопления сырых данных от сервера
	forwardBuffer []byte // Буфер данных, готовых к отправке клиенту (игре)
	mu            sync.Mutex
}

func WrapConn(c net.Conn, ui *AppUI) net.Conn {
	return &Interceptor{Conn: c, ui: ui}
}

func (i *Interceptor) Read(b []byte) (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// 1. Если есть данные в очереди инъекций (наши кнопки), отдаем их первыми
	select {
	case data := <-InjectQueue:
		return copy(b, data), nil
	default:
	}

	// 2. Если в буфере отправки уже лежат нарезанные пакеты — отдаем их
	if len(i.forwardBuffer) > 0 {
		n := copy(b, i.forwardBuffer)
		i.forwardBuffer = i.forwardBuffer[n:]
		return n, nil
	}

	// 3. Читаем новую порцию данных от сервера
	tmp := make([]byte, 8192)
	n, err := i.Conn.Read(tmp)
	if err != nil {
		return n, err
	}

	// Добавляем в накопитель
	i.streamBuf = append(i.streamBuf, tmp[:n]...)

	// 4. ЦИКЛ НАРЕЗКИ: Выделяем ВСЕ пакеты из накопленного буфера
	for len(i.streamBuf) >= 2 {
		// Читаем длину (Little Endian)
		// ВАЖНО: Если длина 14 00 (20 байт) не включает поле длины,
		// то полный размер пакета = pLen + 2
		pLenRaw := binary.LittleEndian.Uint16(i.streamBuf[:2])
		fullPacketLen := int(pLenRaw) + 2

		// Если данных в буфере меньше, чем длина пакета — ждем следующего Read
		if len(i.streamBuf) < fullPacketLen {
			break
		}

		// Извлекаем ОДИН четкий пакет
		packet := make([]byte, fullPacketLen)
		copy(packet, i.streamBuf[:fullPacketLen])

		// Отрезаем извлеченное из основного буфера
		i.streamBuf = i.streamBuf[fullPacketLen:]

		// Отправляем пакет в UI как отдельную строку
		if i.ui.IsRecording.Load() {
			i.ui.AddPacket(ParsePacket(packet))
		}

		// Если НЕ режим паузы — добавляем пакет в буфер для отправки игре
		if !i.ui.IsInterrupt.Load() {
			i.forwardBuffer = append(i.forwardBuffer, packet...)
		}
	}

	// 5. Отдаем игре то, что успели нарезать в forwardBuffer
	if len(i.forwardBuffer) > 0 {
		nForward := copy(b, i.forwardBuffer)
		i.forwardBuffer = i.forwardBuffer[nForward:]
		return nForward, nil
	}

	// Если мы в режиме паузы или пакет еще не дошел целиком
	return 0, nil
}

func SendToClient(hexStr string) {
	data := HexToBytes(hexStr)
	if len(data) > 0 {
		InjectQueue <- data
	}
}
