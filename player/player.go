package player

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
)

type Player struct {
	mu      sync.Mutex
	stream  beep.StreamSeekCloser
	format  beep.Format
	ctrl    *beep.Ctrl
	vol     *effects.Volume
	playing bool
	endedCh chan struct{}
	inited  bool
}

// New 返回一个未加载音轨的播放器
func New() *Player {
	return &Player{
		endedCh: make(chan struct{}),
	}
}

// Load 加载音轨但不自动播放。会根据轨道采样率初始化/重建 speaker。
func (p *Player) Load(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 关闭旧流
	if p.stream != nil {
		_ = p.stream.Close()
		p.stream = nil
	}

	s, format, err := Open(path)
	if err != nil {
		return err
	}

	// 需要按轨道采样率初始化 speaker
	if !p.inited || p.format.SampleRate != format.SampleRate {
		// 100ms 缓冲
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			_ = s.Close()
			return fmt.Errorf("speaker init: %w", err)
		}
		p.inited = true
	}
	p.format = format

	// 包装控制与音量
	p.ctrl = &beep.Ctrl{Streamer: s, Paused: true}
	p.vol = &effects.Volume{
		Streamer: p.ctrl,
		Base:     2,   // 对数底数
		Volume:   0.0, // dB，0 表示 1.0 倍
		Silent:   false,
	}
	p.stream = s

	// 重置 ended 通知通道
	p.endedCh = make(chan struct{})

	p.playing = false
	return nil
}

// Play 从当前位置开始播放
func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream == nil {
		return errors.New("no track loaded")
	}
	if p.playing {
		return nil
	}

	// Clear any existing streams before adding new one
	speaker.Clear()

	p.ctrl.Paused = false
	p.playing = true

	// 组合回调：播放结束时发信号
	speaker.Play(beep.Seq(p.vol, beep.Callback(func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.playing { // 防止 Stop 后重复发送
			p.playing = false
			select {
			case <-p.endedCh:
				// 已关闭则不重复
			default:
				close(p.endedCh)
			}
		}
	})))
	return nil
}

// Pause 暂停播放
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = true
		speaker.Unlock()
		p.playing = false
	}
}

// Toggle 切换播放/暂停
func (p *Player) Toggle() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl == nil {
		return errors.New("no track loaded")
	}
	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	p.playing = !p.ctrl.Paused
	speaker.Unlock()
	return nil
}

// Stop 停止播放并回到开头
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl == nil || p.stream == nil {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = true
	_ = p.stream.Seek(0)
	speaker.Unlock()
	p.playing = false

	// Clear the speaker to stop any ongoing playback
	speaker.Clear()
}

// Seek 定位到指定时间（超界会被截断）
func (p *Player) Seek(pos time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream == nil {
		return errors.New("no track loaded")
	}
	samples := p.format.SampleRate.N(pos)
	speaker.Lock()
	err := p.stream.Seek(samples)
	speaker.Unlock()
	return err
}

// Position 返回当前播放位置
func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream == nil {
		return 0
	}
	// StreamSeekCloser 的 Position() 不是接口方法，但多数解码器都实现了
	type posLen interface {
		Position() int
		Len() int
	}
	if s, ok := p.stream.(posLen); ok {
		return p.format.SampleRate.D(s.Position())
	}
	return 0
}

// Duration 返回音轨总时长（可能为 0，取决于解码器是否可得）
func (p *Player) Duration() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream == nil {
		return 0
	}
	type posLen interface {
		Position() int
		Len() int
	}
	if s, ok := p.stream.(posLen); ok && s.Len() > 0 {
		return p.format.SampleRate.D(s.Len())
	}
	return 0
}

// SetVolume 设置线性音量（0.0~1.0；>1.0 也可但可能失真）
func (p *Player) SetVolume(linear float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.vol == nil {
		return
	}
	if linear <= 0 {
		p.vol.Silent = true
		p.vol.Volume = 0
		return
	}
	p.vol.Silent = false
	// 把线性增益映射成 dB：vol(volume in dB) 满足 Base^Volume = linear
	// 以 Base=2：Volume = log2(linear) = ln(l)/ln(2)
	p.vol.Volume = math.Log(linear) / math.Log(p.vol.Base)
}

// OnEnded 返回一个只读通道，曲目播放完毕会关闭该通道
func (p *Player) OnEnded() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.endedCh
}

// Close 释放当前流
func (p *Player) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream != nil {
		err := p.stream.Close()
		p.stream = nil
		return err
	}
	return nil
}
