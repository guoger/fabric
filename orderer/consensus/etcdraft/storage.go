/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package etcdraft

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
)

// MemoryStorage is currently backed by etcd/raft.MemoryStorage. This interface is
// defined to expose dependencies of fsm so that it may be swapped in the
// future. TODO(jay) Add other necessary methods to this interface once we need
// them in implementation, e.g. ApplySnapshot.
type MemoryStorage interface {
	raft.Storage
	Append(entries []raftpb.Entry) error
	SetHardState(st raftpb.HardState) error
}

type Storage interface {
	Store(entries []raftpb.Entry, hardstate raftpb.HardState) error
	Close() error
}

func Restore(lg *flogging.FabricLogger, applied uint64, walDir string, ram MemoryStorage) (*RaftStorage, bool, error) {
	hasWAL := wal.Exist(walDir)
	if !hasWAL && applied != 0 {
		return nil, false, errors.Errorf("last applied index is non-zero (%d), whereas no WAL data found at %s", applied, walDir)
	}

	if err := mkdir(walDir); err != nil {
		return nil, false, err
	}

	wal, err := replayWAL(lg, hasWAL, walDir, ram)
	if err != nil {
		return nil, false, err
	}

	return &RaftStorage{ram: ram, wal: wal}, false, nil
}

func mkdir(d string) error {
	dir, err := os.Stat(d)
	if err != nil {
		if os.IsNotExist(err) {
			e := os.MkdirAll(d, 0750)
			if e != nil {
				return errors.Errorf("failed to mkdir -p %s: %s", d, e)
			}
		} else {
			return errors.Errorf("failed to stat WALDir %s: %s", d, err)
		}
	} else {
		if !dir.IsDir() {
			return errors.Errorf("%s is not a directory", d)
		}

		if e := isWriteable(d); e != nil {
			return errors.Errorf("WAL directory %s is not writeable: %s", d, e)
		}
	}

	return nil
}

func replayWAL(logger *flogging.FabricLogger, hasWAL bool, walDir string, storage MemoryStorage) (*wal.WAL, error) {
	var w *wal.WAL
	var err error
	if !hasWAL {
		logger.Debugf("No WAL data found, creating new WAL at %s", walDir)
		w, err = wal.Create(walDir, nil)
		if err != nil {
			return nil, errors.Errorf("failed to create new WAL: %s", err)
		}
		w.Close()
	}

	var walsnap walpb.Snapshot // TODO support raft snapshot
	if w, err = wal.Open(walDir, walsnap); err != nil {
		return nil, errors.Errorf("failed to open existing WAL: %s", err)
	}

	_, st, ents, err := w.ReadAll()
	if err != nil {
		return nil, errors.Errorf("failed to read WAL: %s", err)
	}

	logger.Debugf("Setting HardState to {Term: %d, Commit: %d}", st.Term, st.Commit)
	storage.SetHardState(st) // MemoryStorage.SetHardState always returns nil

	logger.Debugf("Appending %d entries to memory storage", len(ents))
	storage.Append(ents) // MemoryStorage.Append always return nil

	return w, nil
}

func isWriteable(dir string) error {
	f := filepath.Join(dir, ".touch")
	if err := ioutil.WriteFile(f, []byte(""), 0700); err != nil {
		return err
	}

	return os.Remove(f)
}

type RaftStorage struct {
	ram MemoryStorage
	wal *wal.WAL
}

func (rs *RaftStorage) Store(entries []raftpb.Entry, hardstate raftpb.HardState) error {
	if err := rs.ram.Append(entries); err != nil {
		return err
	}

	if err := rs.wal.Save(hardstate, entries); err != nil {
		return err
	}

	return nil
}

func (rs *RaftStorage) Close() error {
	if err := rs.wal.Close(); err != nil {
		return err
	}

	return nil
}
