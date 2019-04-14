// import mixpanel from 'mixpanel-browser';

import request from 'common/request';
import { notify } from 'components/Notification';

const fetchEnvironments = () => {
  return request(`{ environments { id,name,endpoints } }`, { 
    auth: true
  });
};

export const requestEnvironments = () => {
  return {
    type: 'REQUEST_ENVIRONMENTS'
  };
};

export const receiveEnvironments = environments => {
  return {
    type: 'RECEIVE_ENVIRONMENTS',
    environments
  };
};

export const handleFetchError = err => {
  notify(`Error: ${err}`, 'error');
  return {
    type: 'HANDLE_FETCH_ERROR'
  };
};

export const refreshEnvironments = () => {
  return dispatch => {
    dispatch(requestEnvironments());
    fetchEnvironments().then(e => {
      dispatch(receiveEnvironments(e.data.environments));
    }).catch(err => dispatch(handleFetchError(err)));
  };
};

export const deleteEnvironment = environment => {
  return dispatch => {
    // mixpanel.track('Delete Environment');

    return request(`mutation {
      down(name: "${environment.name}") {
        name
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshEnvironments());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};
