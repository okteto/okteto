import request from 'common/request';
import { notify } from 'components/Notification';

export const loginWithGithub = token => {
  return dispatch => {
    return request(`/auth/github`, {
      method: 'post',
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify({ token })
    }, {
      responseType: 'json'
    }).then(user => {
      dispatch(authSuccess(user));
    }).catch(err => notify(`Authentication error: ${err}`, 'error'));
  };
};

export const authSuccess = (user) => {
  return {
    type: 'AUTH_SUCCESS',
    user
  };
};

export const logout = () => {
  return { type: 'LOGOUT' };
};

export const afterRestoreSession = (session) => {
  return { 
    type: 'AFTER_RESTORE_SESSION',
    session: session
  };
};

export const saveSession = () => {
  return { type: 'SAVE_SESSION' };
};

export const updateSession = (user) => {
  return { 
    type: 'UPDATE_SESSION',
    user
  };
};

export const refreshSession = () => {
  return (dispatch) => {
    return request(`/users`, {
      method: 'get'
    }, {
      responseType: 'json'
    }).then(user => {
      dispatch(updateSession(user));
    }).catch(err => notify(`Session error: ${err}`, 'error'));
  };
};

export const deleteAccount = () => {
  return (dispatch) => {
    return request(`/users`, {
      method: 'delete',
      headers: {
        "Content-Type": "application/json"
      }
    }).then(() => {
      dispatch(logout());
    }).catch(err => notify(`Authentication error: ${err}`, 'error'));
  };
};