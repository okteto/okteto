import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import classnames from 'classnames';
import autobind from 'autobind-decorator';

import Icon from 'components/Icon';
import CreateSpaceDialog from 'containers/Dialogs/CreateSpace';
import { selectSpace } from 'actions/spaces';
import throttle from 'lodash.throttle';

import './SpaceExplorer.scss';

class SpaceExplorer extends Component {
  constructor(props) {
    super(props);

    this.handleSelectSpace = throttle(this.handleSelectSpace, 1000);
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
    const { spaces, deletingSpaces, user, currentSpace } = this.props;

    return (
      <div className="SpaceExplorer">
        <div className="SpaceExplorerGrid">
          <div className="SpaceExplorerHeader layout vertical center full-width">
            <Icon icon="oktetoHorizontal" size="104" />
          </div>

          <div className="SpaceExplorerList full-width">
            {spaces.map(space => {
              const isPersonalSpace = user.githubID === space.id;

              return (
                <div 
                  key={space.id} 
                  className={classnames('SpaceExplorerListItem layout vertical', {
                    selected: currentSpace.id === space.id,
                    deleting: deletingSpaces.includes(space.id)
                  })}
                  title={space.id}
                  onClick={() => this.handleSelectSpace(space)}
                >
                  <div className="ellipsis">{ space.id }</div>
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
  deletingSpaces: PropTypes.arrayOf(PropTypes.string).isRequired,
  currentSpace: PropTypes.object.isRequired,
  user: PropTypes.object.isRequired,
  dispatch: PropTypes.func
};

export default ReactRedux.connect(state => {
  return {
    spaces: state.spaces.list,
    deletingSpaces: state.spaces.deleting,
    currentSpace: state.spaces.current,
    user: state.session.user,
  };
})(SpaceExplorer);