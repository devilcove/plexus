// Code generated by "stringer -type Command"; DO NOT EDIT.

package plexus

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[DeletePeer-0]
	_ = x[AddPeer-1]
	_ = x[UpdatePeer-2]
	_ = x[AddRelay-3]
	_ = x[DeleteRelay-4]
	_ = x[DeleteNetwork-5]
	_ = x[JoinNetwork-6]
	_ = x[LeaveNetwork-7]
	_ = x[LeaveServer-8]
	_ = x[Ping-9]
}

const _Command_name = "DeletePeerAddPeerUpdatePeerAddRelayDeleteRelayDeleteNetworkJoinNetworkLeaveNetworkLeaveServerPing"

var _Command_index = [...]uint8{0, 10, 17, 27, 35, 46, 59, 70, 82, 93, 97}

func (i Command) String() string {
	if i < 0 || i >= Command(len(_Command_index)-1) {
		return "Command(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Command_name[_Command_index[i]:_Command_index[i+1]]
}
