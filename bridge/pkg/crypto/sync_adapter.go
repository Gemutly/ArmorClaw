package crypto

import (
	"encoding/json"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func BridgeSyncToMautrix(bridgeResp *SyncResponse) *mautrix.RespSync {
	if bridgeResp == nil {
		return &mautrix.RespSync{}
	}

	resp := &mautrix.RespSync{}

	if bridgeResp.ToDevice != nil && len(bridgeResp.ToDevice.Events) > 0 {
		resp.ToDevice = mautrix.SyncEventsList{
			Events: parseRawEvents(bridgeResp.ToDevice.Events),
		}
	}

	if bridgeResp.DeviceLists != nil {
		resp.DeviceLists = mautrix.DeviceLists{
			Changed: convertUserIDs(bridgeResp.DeviceLists.Changed),
			Left:    convertUserIDs(bridgeResp.DeviceLists.Left),
		}
	}

	if len(bridgeResp.DeviceOneTimeKeysCount) > 0 {
		resp.DeviceOTKCount = mapOTKCount(bridgeResp.DeviceOneTimeKeysCount)
	}

	return resp
}

func parseRawEvents(rawEvents []json.RawMessage) []*event.Event {
	events := make([]*event.Event, 0, len(rawEvents))
	for _, raw := range rawEvents {
		var evt event.Event
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}
		events = append(events, &evt)
	}
	return events
}

func convertUserIDs(strings []string) []id.UserID {
	if len(strings) == 0 {
		return nil
	}
	userIDs := make([]id.UserID, len(strings))
	for i, s := range strings {
		userIDs[i] = id.UserID(s)
	}
	return userIDs
}

func mapOTKCount(counts map[string]int) mautrix.OTKCount {
	otk := mautrix.OTKCount{}
	for alg, count := range counts {
		switch alg {
		case "curve25519":
			otk.Curve25519 = count
		case "signed_curve25519":
			otk.SignedCurve25519 = count
		}
	}
	return otk
}

func AdapterSyncResponse(toDevice *ToDeviceSync, deviceLists *DeviceListsSync, otkCount map[string]int) *SyncResponse {
	return &SyncResponse{
		ToDevice:               toDevice,
		DeviceLists:            deviceLists,
		DeviceOneTimeKeysCount: otkCount,
	}
}

func FromAdapterToDeviceSync(events []json.RawMessage) *ToDeviceSync {
	if len(events) == 0 {
		return nil
	}
	return &ToDeviceSync{Events: events}
}

func FromAdapterDeviceList(changed, left []string) *DeviceListsSync {
	if len(changed) == 0 && len(left) == 0 {
		return nil
	}
	return &DeviceListsSync{Changed: changed, Left: left}
}

func ConvertToDeviceEvents(rawEvents []json.RawMessage) []event.Event {
	var result []event.Event
	for _, raw := range rawEvents {
		var evt event.Event
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}
		switch evt.Type {
		case event.ToDeviceRoomKey, event.ToDeviceRoomKeyRequest,
			event.ToDeviceForwardedRoomKey, event.ToDeviceRoomKeyWithheld,
			event.ToDeviceEncrypted:
			result = append(result, evt)
		}
	}
	return result
}
