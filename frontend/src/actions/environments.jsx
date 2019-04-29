import request from 'common/request';
import analytics from 'common/analytics';
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

export const failedReceiveEnvironments = err => {
  notify(`Error: ${err}`, 'error');
  return {
    type: 'FAILED_RECEIVE_ENVIRONMENTS'
  };
};

export const refreshEnvironments = () => {
  return dispatch => {
    dispatch(requestEnvironments());
    fetchEnvironments().then(e => {
      dispatch(receiveEnvironments(e.data.environments));
    }).catch(err => dispatch(failedReceiveEnvironments(err)));
  };
};

export const deleteEnvironment = environment => {
  return dispatch => {
    analytics.track('Delete Environment');

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
