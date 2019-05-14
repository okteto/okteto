import request from 'common/request';
import analytics from 'common/analytics';
import { notify } from 'components/Notification';

const getSpacesQuery = `spaces {
  id, name 
}`;

const getSpaceQuery = `space(id: $space) {
  id, name, members {
    id, githubID, avatar, name, owner
  }
}
environments(space: $space) {
  id, name, space, endpoints, dev { 
    id, name, githubID, avatar, owner
  }
}
databases(space: $space) { 
  name, space, endpoint 
}`;


const fetchSpaces = () => {
  return request(`
    query GetSpaces {
      ${getSpacesQuery}
    }
  `).then(response => response.spaces);
};

const fetchSpace = spaceId => {
  return request(`
    query GetSpace($space: String!) {
      ${getSpaceQuery}
    }
  `, {
    space: spaceId
  }).then(response => {
    return {
      ...response.space || {},
      environments: response.environments || [],
      databases: response.databases || []
    };
  });
};

const sortSpaces = (spaces, user) => {
  return spaces.sort((a, b) => {
    // Personal space should be placed first.
    if (a.id === user.id) return -1;
    if (b.id === user.id) return 1;
    var nameA = a.name.toLowerCase();
    var nameB = b.name.toLowerCase();
    if (nameA < nameB) return -1;
    if (nameA > nameB) return 1;
    return 0;
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

export const requestSpace = () => {
  return {
    type: 'REQUEST_SPACE'
  };
};

export const receiveSpace = space => {
  return {
    type: 'RECEIVE_SPACE',
    space
  };
};

export const failedReceiveSpaces = err => {
  notify(`Error: ${err}`, 'error');
  return {
    type: 'FAILED_RECEIVE_SPACES'
  };
};

export const failedReceiveSpace = err => {
  notify(`Error: ${err}`, 'error');
  return {
    type: 'FAILED_RECEIVE_SPACE'
  };
};

export const discardReceiveSpace = spaceId => {
  return {
    type: 'DISCARD_RECEIVE_SPACE',
    spaceId
  };
};

export const changeCurrentSpace = spaceId => {
  return {
    type: 'CHANGE_CURRENT_SPACE',
    spaceId
  };
};

export const selectSpace = spaceId => {
  return dispatch => {
    dispatch(changeCurrentSpace(spaceId));
    dispatch(refreshCurrentSpace());
  }
};

export const refreshSpaces = () => {
  return (dispatch, getState) => {
    const { spaces, session } = getState();

    if (!spaces.isFetching) {
      dispatch(requestSpaces());
      fetchSpaces().then(newSpaces => {
        dispatch(receiveSpaces(sortSpaces(newSpaces, session.user)));

        // Clean deleted spaces.
        const { spaces } = getState();
        for (const deletingSpace of spaces.deleting) {
          if (!newSpaces.find(space => space.id === deletingSpace.id)) {
            dispatch(deletedSpace(deletingSpace.id));
          }
        }
      }).catch(err => dispatch(failedReceiveSpaces(err)));
    }
  };
};

export const refreshCurrentSpace = () => {
  return (dispatch, getState) => {
    const { spaces, session } = getState();

    // If no selected space, use personal space.
    const currentSpaceId = spaces.currentId ? spaces.currentId : session.user.id;

    dispatch(requestSpace());
    fetchSpace(currentSpaceId).then(space => {
      const { spaces } = getState();

      // Only accept those that match current selected space.
      if (!space.current || space.id !== spaces.currentId) {
        dispatch(receiveSpace(space));
      } else {
        dispatch(discardReceiveSpace());
      }
    }).catch(err => dispatch(failedReceiveSpace(err)));
  }
};

export const createSpace = (name, members = []) => {
  return dispatch => {
    analytics.track('Create Space');

    return request(`mutation CreateSpace($name: String!, $members: [String]) {
      createSpace(name: $name, members: $members) {
        id
      }
    }`, {
      name,
      members
    }).then(response => {
      const spaceId = response.createSpace.id;
      dispatch(refreshSpaces());
      dispatch(selectSpace(spaceId));
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const createDatabase = (spaceId, name) => {
  return dispatch => {
    analytics.track('Create Database');

    return request(`mutation CreateDatabase($space: String!, $name: String!) {
      createDatabase(space: $space, name: $name) {
        id
      }
    }`, {
      space: spaceId,
      name
    }).then(() => {
      dispatch(refreshCurrentSpace());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteDatabase = database => {
  return dispatch => {
    analytics.track('Delete Database');

    return request(`mutation DeleteDatabase($space: String!, $name: String!){
      deleteDatabase(space: $space, name: $name) {
        name
      }
    }`, {
      space: database.space,
      name: database.name
    }).then(() => {
      dispatch(refreshCurrentSpace());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteEnvironment = environment => {
  return dispatch => {
    analytics.track('Delete Environment');

    return request(`mutation DeleteEnvironment($space: String!, $name: String!) {
      down(space: $space, name: $name) {
        name
      }
    }`, {
      space: environment.space,
      name: environment.name
    }).then(() => {
      dispatch(refreshCurrentSpace());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteSpace = space => {
  return (dispatch, getState) => {
    analytics.track('Delete Space');

    return request(`mutation DeleteSpace($space: String!) { 
      deleteSpace(id: $space) {
        name
      } 
    }`, {
      space: space.id
    }).then(() => {
      const { session } = getState();
      // Select personal space.
      dispatch(selectSpace(session.user.id));
      dispatch(deletingSpace(space.id));
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const leaveSpace = space => {
  return (dispatch, getState) => {
    analytics.track('Leave Space');

    return request(`mutation LeaveSpace($space: String!) { 
      leaveSpace(id: $space) {
        id
      } 
    }`, {
      space: space.id
    }).then(() => {
      const { session } = getState();
      // Select personal space.
      dispatch(selectSpace(session.user.id));
      dispatch(deletingSpace(space.id));
      dispatch(refreshSpaces());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deletingSpace = spaceId => {
  return {
    type: 'DELETING_SPACE',
    spaceId
  };
};

export const deletedSpace = spaceId => {
  return {
    type: 'DELETED_SPACE',
    spaceId
  };
};

export const shareSpace = (spaceId, members) => {
  return dispatch => {
    analytics.track('Share Space');

    return request(`mutation ShareSpace($space: String!, $members: [String]) {
      updateSpace(id: $space, members: $members) {
        id
      }
    }`, {
      space: spaceId,
      members
    }).then(() => {
      dispatch(refreshCurrentSpace());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};
