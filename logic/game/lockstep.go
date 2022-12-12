package game

import (
	"github.com/hedon954/go-lock-step-server/pb"
)

type frameData struct {
	idx  uint32
	cmds []*pb.InputData
}

func newFrameData(index uint32) *frameData {
	return &frameData{
		idx:  index,
		cmds: make([]*pb.InputData, 0),
	}
}

type lockstep struct {
	frames     map[uint32]*frameData
	frameCount uint32
}

func newLockStep() *lockstep {
	return &lockstep{
		frames: make(map[uint32]*frameData),
	}
}

func (l *lockstep) reset() {
	l.frames = make(map[uint32]*frameData)
	l.frameCount = 0
}

func (l *lockstep) getFrameCount() uint32 {
	return l.frameCount
}

func (l *lockstep) tick() uint32 {
	l.frameCount++
	return l.frameCount
}

func (l *lockstep) getFrame(idx uint32) *frameData {
	return l.frames[idx]
}

func (l *lockstep) getRangeFrames(from, to uint32) []*frameData {
	ret := make([]*frameData, 0, to-from)
	for ; from <= to && from <= l.frameCount; from++ {
		f, ok := l.frames[from]
		if !ok {
			continue
		}
		ret = append(ret, f)
	}
	return ret
}

func (l *lockstep) pushCmd(cmd *pb.InputData) bool {
	f, ok := l.frames[l.frameCount]
	if !ok {
		f = newFrameData(l.frameCount)
		l.frames[l.frameCount] = f
	}

	// check if the same frame sent two operations
	for _, c := range f.cmds {
		if c.Id == cmd.Id {
			return false
		}
	}
	f.cmds = append(f.cmds, cmd)
	return true
}
