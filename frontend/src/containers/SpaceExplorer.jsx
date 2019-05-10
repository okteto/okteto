import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import classnames from 'classnames';
import autobind from 'autobind-decorator';

import Icon from 'components/Icon';
import CreateSpaceDialog from 'containers/Dialogs/CreateSpace';
import { selectSpace } from 'actions/spaces';

import './SpaceExplorer.scss';

class SpaceExplorer extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleCreateSpace() {
    this.createSpaceDialog.getWrappedInstance().open();
  }

  @autobind
  handleSelectSpace(space) {
    this.props.dispatch(selectSpace(space.id));
  }

  render() {
    const { spaces, user, currentSpace } = this.props;

    return (
      <div className="SpaceExplorer">
        <div className="SpaceExplorerGrid">
          <div className="SpaceExplorerHeader layout vertical center">
            <Icon icon="oktetoHorizontal" size="104" />
          </div>

          <div className="SpaceExplorerList">
            {spaces.map(space => {
              const spaceName = user.id === space.id ? `${user.githubID}'s space` : space.name;

              return (
                <div 
                  key={space.id} 
                  className={classnames('SpaceExplorerListItem ellipsis', {
                    selected: currentSpace.id === space.id
                  })}
                  title={spaceName}
                  onClick={() => this.handleSelectSpace(space)}
                >
                  { spaceName }
                </div>
              );
            })}
          </div>
        </div>

        <CreateSpaceDialog ref={ref => this.createSpaceDialog = ref} />
        
        <div className="SpaceExplorerNewButton" onClick={this.handleCreateSpace}>
          <Icon
            icon="plus"
            size="20"
          />
        </div>
      </div>
    );  
  }
}

SpaceExplorer.propTypes = {
  spaces: PropTypes.arrayOf(PropTypes.object).isRequired,
  currentSpace: PropTypes.object.isRequired,
  user: PropTypes.object.isRequired,
  dispatch: PropTypes.func
};

export default ReactRedux.connect(state => {
  return {
    spaces: state.spaces.list,
    currentSpace: state.spaces.current,
    user: state.session.user,
  };
})(SpaceExplorer);