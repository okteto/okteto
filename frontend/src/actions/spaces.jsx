import request from 'common/request';
import analytics from 'common/analytics';
import { notify } from 'components/Notification';

const fetchAll = (spaceId) => {
  return request(`{
    spaces {
      id, name 
    }
    space(id: "${spaceId}") {
      id, name, members {
        id, githubID, avatar, name, owner
      }
    }
    environments(space: "${spaceId}") {
      id, name, space, endpoints, dev { 
        id, name, githubID, avatar, owner
      }
    }
    databases(space: "${spaceId}") { 
      name, space, endpoint 
    }
  }`, {
    auth: true
  }).then(response => {
    return {
      space: {
        ...response.data.space || {},
        environments: response.data.environments || [],
        databases: response.data.databases || []
      },
      spaces: response.data.spaces
    };
  });
};

export const requestSpaces = () => {
  return {
    type: 'REQUEST_SPACES'
  };
};

export const receiveSpaces = spaces => {
  return {
    type: 'RECEIVE_SPACES',
    spaces
  };
};

export const receiveSelectedSpace = space => {
  return {
    type: 'RECEIVE_SELECTED_SPACE',
    space
  };
};

export const failedReceiveSpaces = err => {
  notify(`Error: ${err}`, 'error');
  return {
    type: 'FAILED_RECEIVE_SPACES'
  };
};

export const refreshSpaces = () => {
  return (dispatch, getState) => {
    const { spaces, session } = getState();
    const currentSpaceId = spaces.current ? spaces.current.id : session.user.id;

    if (!spaces.isFetching) {
      dispatch(requestSpaces());
      fetchAll(currentSpaceId).then(({ spaces, space }) => {
        dispatch(receiveSpaces(spaces));
        dispatch(receiveSelectedSpace(space));
      }).catch(err => dispatch(failedReceiveSpaces(err)));
    }
  };
};

export const selectSpace = spaceId => {
  return (dispatch, getState) => {
    const { spaces } = getState();

    if (!spaces.isFetching) {
      dispatch(requestSpaces());
      fetchAll(spaceId).then(({ spaces, space }) => {
        dispatch(receiveSpaces(spaces));
        dispatch(receiveSelectedSpace(space));
      }).catch(err => dispatch(failedReceiveSpaces(err)));
    }
  };
};

export const createSpace = (name) => {
  return dispatch => {
    analytics.track('Create Space');

    return request(`mutation {
      createSpace(name: "${name}", members: []) {
        id
      }
    }`, {
      auth: true
    }).then(response => {
      const spaceId = response.data.createSpace.id;
      // TODO: Select new created space.
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const createDatabase = (spaceId, name) => {
  return dispatch => {
    analytics.track('Create Database');

    return request(`mutation {
      createDatabase(space: "${spaceId}", name: "${name}") {
        id
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteDatabase = database => {
  return dispatch => {
    analytics.track('Delete Database');

    return request(`mutation {
      deleteDatabase(space: "${database.space}", name: "${database.name}") {
        name
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteEnvironment = environment => {
  return dispatch => {
    analytics.track('Delete Environment');

    return request(`mutation {
      down(space: "${environment.space}", name: "${environment.name}") {
        name
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteSpace = space => {
  return dispatch => {
    analytics.track('Delete Space');

    return request(`mutation { 
      deleteSpace(id: "${space.id}") {
        name
      } 
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const shareSpace = (spaceId, members) => {
  return dispatch => {
    analytics.track('Create Database');

    const queryArray = members.reduce(
      (query, member, i) => query + `"${member}"${i < members.length-1 ? ',' : ''}`, '');

    return request(`mutation {
      updateSpace(id: "${spaceId}", members: [${queryArray}]) {
        id
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};
