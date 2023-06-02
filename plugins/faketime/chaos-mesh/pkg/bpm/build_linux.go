// Copyright 2021 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package bpm

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/chaos-mesh/chaos-mesh/pkg/mock"
)

// Build builds the command
// the ctx argument passes the context information to this function
// e.g. the corresponding resource name.
func (b *CommandBuilder) Build(ctx context.Context) *ManagedCommand {
	log := b.getLoggerFromContext(ctx)
	args := b.args
	cmd := b.cmd

	if len(b.nsOptions) > 0 {
		args = append([]string{"--", cmd}, args...)
		for _, option := range b.nsOptions {
			args = append([]string{"-" + nsArgMap[option.Typ], option.Path}, args...)
		}

		if b.localMnt {
			args = append([]string{"-l"}, args...)
		}
		cmd = nsexecPath
	}

	if b.oomScoreAdj != 0 {
		args = append([]string{"-n", strconv.Itoa(b.oomScoreAdj), "--", cmd}, args...)
		cmd = "choom"
	}

	// pause should always be the first command to execute because the
	// `stress_server` will check whether the /proc/PID/comm is `pause` to
	// determine whether it should continue to send `SIGCONT`. If the first
	// command is not `pause`, the real `pause` program may not receive the
	// command successfully.
	if b.pause {
		args = append([]string{cmd}, args...)
		cmd = pausePath
	}

	if c := mock.On("MockProcessBuild"); c != nil {
		f := c.(func(context.Context, string, ...string) *exec.Cmd)
		return &ManagedCommand{
			Cmd:        f(b.ctx, cmd, args...),
			Identifier: b.identifier,
		}
	}

	log.Info("build command", "command", cmd+" "+strings.Join(args, " "))

	command := exec.CommandContext(b.ctx, cmd, args...)
	command.Env = b.env
	command.SysProcAttr = &syscall.SysProcAttr{}
	command.SysProcAttr.Pdeathsig = syscall.SIGTERM

	if b.stdin != nil {
		command.Stdin = b.stdin
	}

	if b.stdout != nil {
		command.Stdout = b.stdout
	}

	if b.stderr != nil {
		command.Stderr = b.stderr
	}

	return &ManagedCommand{
		Cmd:        command,
		Identifier: b.identifier,
	}
}
