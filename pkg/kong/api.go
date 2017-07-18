package kong

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/sirupsen/logrus"
)

// GetNodeInformation get Kong node informations
func (k *Client) GetNodeInformation() (*NodeInformationResponse, error) {
	res, err := k.get("/", nil)
	if err != nil {
		logrus.WithError(err).Error("unable to retrieve server version")
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		err := errors.New(res.Status)
		logrus.WithError(err).Error("unable to retrieve server version")
		return nil, err
	}

	var result NodeInformationResponse
	err = json.NewDecoder(res.Body).Decode(&result)
	return &result, err
}

// RetrieveUpstream get upstream by its name
func (k *Client) RetrieveUpstream(name string) (*UpstreamResponse, error) {
	res, err := k.get("/upstreams/"+name, nil)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"name": name,
		}).Error("unable to retrieve upstream")
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		logrus.WithFields(logrus.Fields{
			"name": name,
		}).Debug("upstream not found")
		return nil, nil
	}
	if res.StatusCode >= 400 {
		logrus.WithFields(logrus.Fields{
			"name": name,
		}).Error("unable to retrieve upstream")
		return nil, errors.New(res.Status)
	}

	var result UpstreamResponse
	err = json.NewDecoder(res.Body).Decode(&result)
	return &result, err
}

// UpdateOrCreateUpstream update or create upstream
func (k *Client) UpdateOrCreateUpstream(name string) error {
	req := &UpstreamRequest{Name: name}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(req)
	res, err := k.post("/upstreams", nil, buf)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"name": name,
		}).Error("unable to update or create upstream")
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		logrus.WithFields(logrus.Fields{
			"name": name,
		}).Error("unable to update or create upstream")
		return errors.New(res.Status)
	}
	return err
}

// DeleteUpstream delete upstream
func (k *Client) DeleteUpstream(name string) error {
	res, err := k.delete("/upstreams/"+name, nil)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"name": name,
		}).Error("unable to delete upstream")
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		logrus.WithFields(logrus.Fields{
			"name": name,
		}).Error("unable to delete upstream")
		return errors.New(res.Status)
	}
	return err
}

// ListActiveTargets get active target list
func (k *Client) ListActiveTargets(upstream string) (*TargetListResponse, error) {
	res, err := k.get("/upstreams/"+upstream+"/targets/active", nil)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream": upstream,
		}).Error("unable to fetch upstream targets")
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream": upstream,
		}).Error("unable to fetch upstream targets")
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
		}).Error("unable to fetch upstream targets")
		return nil, errors.New(res.Status)
	}
	// FIXME
	// We have to verify the result size before unmarshal
	// because Kong API is not consitent and return an empty
	// object instead of an empty array if no results.
	var lightResult LightTargetListResponse
	err = json.Unmarshal(body, &lightResult)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream": upstream,
		}).Error("unable to decode response")
		return nil, err
	}
	var result TargetListResponse
	if lightResult.Total == 0 {
		result = TargetListResponse{
			Total: 0,
			Data:  make([]TargetResponse, 0, 0),
		}
	} else {
		err = json.Unmarshal(body, &result)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"upstream": upstream,
			}).Error("unable to decode response")
			return nil, err
		}
	}

	return &result, nil
}

// AddTarget add target to upstream
func (k *Client) AddTarget(upstream, target string, weight uint) (*TargetResponse, error) {
	req := &TargetRequest{Target: target, Weight: weight}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(req)
	res, err := k.post("/upstreams/"+upstream+"/targets", nil, buf)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream": upstream,
			"target":   target,
		}).Error("unable to add target to upstream")
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
			"target":   target,
		}).Error("unable to add target to upstream")
		return nil, errors.New(res.Status)
	}
	var result TargetResponse
	err = json.NewDecoder(res.Body).Decode(&result)
	return &result, err
}

// DeleteTarget delete target
func (k *Client) DeleteTarget(upstream, targetID string) error {
	res, err := k.delete("/upstreams/"+upstream+"/targets/"+targetID, nil)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream":  upstream,
			"target_id": targetID,
		}).Error("unable to delete target from upstream")
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		logrus.WithFields(logrus.Fields{
			"upstream":  upstream,
			"target_id": targetID,
		}).Debug("unable to delete target from upstream: target not found")
		return nil
	}
	if res.StatusCode >= 400 {
		return errors.New(res.Status)
	}
	return err
}
