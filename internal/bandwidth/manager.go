package bandwidth

import (
	"context"
	"io"
	"sync"

	"golang.org/x/time/rate"
)

const maxBurst = 64 << 10

type Manager struct {
	mu       sync.Mutex
	limiters map[int64]*rate.Limiter
}

func NewManager() *Manager { return &Manager{limiters: make(map[int64]*rate.Limiter)} }

func (m *Manager) Limiter(userID, bytesPerSecond int64) *rate.Limiter {
	if bytesPerSecond < 1 {
		bytesPerSecond = 1
	}
	burst := min(int(bytesPerSecond), maxBurst)
	m.mu.Lock()
	defer m.mu.Unlock()
	limiter := m.limiters[userID]
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Limit(bytesPerSecond), burst)
		m.limiters[userID] = limiter
		return limiter
	}
	limiter.SetLimit(rate.Limit(bytesPerSecond))
	limiter.SetBurst(burst)
	return limiter
}

func Copy(ctx context.Context, limiter *rate.Limiter, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, min(limiter.Burst(), 32<<10))
	var total int64
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			if err := limiter.WaitN(ctx, n); err != nil {
				return total, err
			}
			written, err := dst.Write(buffer[:n])
			total += int64(written)
			if err != nil {
				return total, err
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}
